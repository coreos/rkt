// Copyright 2014 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/coreos/rocket/Godeps/_workspace/src/github.com/appc/spec/schema"
	"github.com/coreos/rocket/Godeps/_workspace/src/github.com/appc/spec/schema/types"
	"github.com/coreos/rocket/Godeps/_workspace/src/github.com/gorilla/mux"
	"github.com/coreos/rocket/common"
)

var (
	cmdMetadataService = &Command{
		Name:    "metadata-service",
		Summary: "Run metadata service",
		Usage:   "[--src-addr CIDR] [--listen-port PORT] [--no-idle]",
		Run:     runMetadataService,
	}
)

type mdsContainer struct {
	uuid     types.UUID
	manifest schema.PodManifest
	apps     map[string]*schema.ImageManifest
	ip       string
}

var (
	containerByIP  = make(map[string]*mdsContainer)
	containerByUID = make(map[types.UUID]*mdsContainer)
	mutex          = sync.Mutex{}
	hmacKey        [sha512.Size]byte

	flagListenPort int
	flagNoIdle     bool

	exitCh = make(chan os.Signal, 1)
)

const (
	listenFdsStart = 3
)

func init() {
	commands = append(commands, cmdMetadataService)
	cmdMetadataService.Flags.IntVar(&flagListenPort, "listen-port", common.MetadataServicePort, "listen port")
	cmdMetadataService.Flags.BoolVar(&flagNoIdle, "no-idle", false, "exit when last container is unregistered")
}

func queryValue(u *url.URL, key string) string {
	vals, ok := u.Query()[key]
	if !ok || len(vals) != 1 {
		return ""
	}
	return vals[0]
}

func handleRegisterContainer(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	uuid, err := types.NewUUID(mux.Vars(r)["uuid"])
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "UUID is missing or malformed: %v", err)
		return
	}

	containerIP := queryValue(r.URL, "ip")
	if containerIP == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "ip missing")
		return
	}

	c := &mdsContainer{
		apps: make(map[string]*schema.ImageManifest),
		ip:   containerIP,
	}

	if err := json.NewDecoder(r.Body).Decode(&c.manifest); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "JSON-decoding failed: %v", err)
		return
	}

	mutex.Lock()
	containerByIP[containerIP] = c
	containerByUID[*uuid] = c
	mutex.Unlock()

	w.WriteHeader(http.StatusOK)
}

func handleUnregisterContainer(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	uuid, err := types.NewUUID(mux.Vars(r)["uuid"])
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "UUID is missing or malformed: %v", err)
		return
	}

	var lastOne bool
	err = func() error {
		mutex.Lock()
		defer mutex.Unlock()

		c, ok := containerByUID[*uuid]
		if !ok {
			return fmt.Errorf("Container with given UUID not found")
		}

		delete(containerByUID, *uuid)
		delete(containerByIP, c.ip)

		lastOne = (len(containerByUID) == 0)
		return nil
	}()

	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)

	if flagNoIdle && lastOne {
		// TODO(eyakubovich): this is very racy
		// It's possible for last container to get unregistered
		// and svc gets flagged to shutdown. Then another container
		// starts to launch, sees that port is in use and doesn't
		// start metadata svc only for this one to exit a moment later.
		// However, --no-idle is meant for demos and having a single
		// container spawn up (via --spawn-metadata-svc). The design
		// of metadata svc is also likely to change as we convert it
		// to be backed by persistent storage.
		// wait for signal and exit
		close(exitCh)
	}
}

func handleRegisterApp(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	uuid, err := types.NewUUID(mux.Vars(r)["uuid"])
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "UUID is missing or mulformed: %v", err)
		return
	}

	an := mux.Vars(r)["app"]

	app := &schema.ImageManifest{}
	if err := json.NewDecoder(r.Body).Decode(&app); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "JSON-decoding failed: %v", err)
		return
	}

	err = func() error {
		mutex.Lock()
		defer mutex.Unlock()

		c, ok := containerByUID[*uuid]
		if !ok {
			return fmt.Errorf("Container with given UUID not found")
		}

		c.apps[an] = app
		return nil
	}()

	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func containerGet(h func(w http.ResponseWriter, r *http.Request, c *mdsContainer)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		remoteIP := strings.Split(r.RemoteAddr, ":")[0]

		mutex.Lock()
		c, ok := containerByIP[remoteIP]
		mutex.Unlock()

		if !ok {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, "container by remoteIP (%v) not found", remoteIP)
			return
		}

		h(w, r, c)
	}
}

func appGet(h func(w http.ResponseWriter, r *http.Request, c *mdsContainer, _ *schema.ImageManifest)) http.HandlerFunc {
	return containerGet(func(w http.ResponseWriter, r *http.Request, c *mdsContainer) {
		appname := mux.Vars(r)["app"]

		mutex.Lock()
		im, ok := c.apps[appname]
		mutex.Unlock()

		if ok {
			h(w, r, c, im)
		} else {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, "App (%v) not found", appname)
		}
	})
}

func handleContainerAnnotations(w http.ResponseWriter, r *http.Request, c *mdsContainer) {
	defer r.Body.Close()

	w.Header().Add("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)

	for k := range c.manifest.Annotations {
		fmt.Fprintln(w, k)
	}
}

func handleContainerAnnotation(w http.ResponseWriter, r *http.Request, c *mdsContainer) {
	defer r.Body.Close()

	k, err := types.NewACName(mux.Vars(r)["name"])
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "Container annotation is not a valid AC Name")
		return
	}

	v, ok := c.manifest.Annotations.Get(k.String())
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "Container annotation (%v) not found", k)
		return
	}

	w.Header().Add("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(v))
}

func handlePodManifest(w http.ResponseWriter, r *http.Request, c *mdsContainer) {
	defer r.Body.Close()

	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(c.manifest); err != nil {
		log.Print(err)
	}
}

func handleContainerUUID(w http.ResponseWriter, r *http.Request, c *mdsContainer) {
	defer r.Body.Close()

	uuid := c.uuid.String()

	w.Header().Add("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(uuid))
}

func mergeAppAnnotations(im *schema.ImageManifest, cm *schema.PodManifest) types.Annotations {
	merged := types.Annotations{}

	for _, annot := range im.Annotations {
		merged.Set(annot.Name, annot.Value)
	}

	if app := cm.Apps.Get(im.Name); app != nil {
		for _, annot := range app.Annotations {
			merged.Set(annot.Name, annot.Value)
		}
	}

	return merged
}

func handleAppAnnotations(w http.ResponseWriter, r *http.Request, c *mdsContainer, im *schema.ImageManifest) {
	defer r.Body.Close()

	w.Header().Add("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)

	for _, annot := range mergeAppAnnotations(im, &c.manifest) {
		fmt.Fprintln(w, string(annot.Name))
	}
}

func handleAppAnnotation(w http.ResponseWriter, r *http.Request, c *mdsContainer, im *schema.ImageManifest) {
	defer r.Body.Close()

	k, err := types.NewACName(mux.Vars(r)["name"])
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "App annotation is not a valid AC Name")
		return
	}

	merged := mergeAppAnnotations(im, &c.manifest)

	v, ok := merged.Get(k.String())
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "App annotation (%v) not found", k)
		return
	}

	w.Header().Add("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(v))
}

func handleImageManifest(w http.ResponseWriter, r *http.Request, c *mdsContainer, im *schema.ImageManifest) {
	defer r.Body.Close()

	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(*im); err != nil {
		log.Print(err)
	}
}

func handleAppID(w http.ResponseWriter, r *http.Request, c *mdsContainer, im *schema.ImageManifest) {
	defer r.Body.Close()

	w.Header().Add("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	a := c.manifest.Apps.Get(im.Name)
	if a == nil {
		panic("could not find app in manifest!")
	}
	w.Write([]byte(a.Image.ID.String()))
}

func initCrypto() error {
	if n, err := rand.Reader.Read(hmacKey[:]); err != nil || n != len(hmacKey) {
		return fmt.Errorf("Failed to generate HMAC Key")
	}
	return nil
}

func handleContainerSign(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	remoteIP := strings.Split(r.RemoteAddr, ":")[0]

	mutex.Lock()
	c, ok := containerByIP[remoteIP]
	mutex.Unlock()

	if !ok {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "Metadata by remoteIP (%v) not found", remoteIP)
		return
	}

	content := r.FormValue("content")
	if content == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "content form value not found")
		return
	}

	// HMAC(UID:content)
	h := hmac.New(sha512.New, hmacKey[:])
	h.Write(c.uuid[:])
	h.Write([]byte(content))

	// Send back HMAC as the signature
	w.Header().Add("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	enc := base64.NewEncoder(base64.StdEncoding, w)
	enc.Write(h.Sum(nil))
	enc.Close()
}

func handleContainerVerify(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	uid, err := types.NewUUID(r.FormValue("uid"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "uid field missing or malformed: %v", err)
		return
	}

	content := r.FormValue("content")
	if content == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "content field missing")
		return
	}

	sig, err := base64.StdEncoding.DecodeString(r.FormValue("signature"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "signature field missing or corrupt: %v", err)
		return
	}

	h := hmac.New(sha512.New, hmacKey[:])
	h.Write(uid[:])
	h.Write([]byte(content))

	if hmac.Equal(sig, h.Sum(nil)) {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusForbidden)
	}
}

type httpResp struct {
	writer http.ResponseWriter
	status int
}

func (r *httpResp) Header() http.Header {
	return r.writer.Header()
}

func (r *httpResp) Write(d []byte) (int, error) {
	return r.writer.Write(d)
}

func (r *httpResp) WriteHeader(status int) {
	r.status = status
	r.writer.WriteHeader(status)
}

func logReq(h func(w http.ResponseWriter, r *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp := &httpResp{w, 0}
		h(resp, r)
		log.Printf("%v %v - %v", r.Method, r.RequestURI, resp.status)
	}
}

// unixListener returns the listener used for registrations (over unix sock)
func unixListener() (net.Listener, error) {
	s := os.Getenv("LISTEN_FDS")
	if s != "" {
		// socket activated
		lfds, err := strconv.ParseInt(s, 10, 16)
		if err != nil {
			return nil, fmt.Errorf("Error parsing LISTEN_FDS env var: %v", err)
		}
		if lfds < 1 {
			return nil, fmt.Errorf("LISTEN_FDS < 1")
		}

		return net.FileListener(os.NewFile(uintptr(listenFdsStart), "listen"))
	} else {
		dir := filepath.Dir(common.MetadataServiceRegSock)
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			return nil, fmt.Errorf("Failed to create %v: %v", dir, err)
		}

		return net.ListenUnix("unix", &net.UnixAddr{
			Net:  "unix",
			Name: common.MetadataServiceRegSock,
		})
	}
}

func runRegistrationServer(l net.Listener) {
	r := mux.NewRouter()
	r.HandleFunc("/pods/{uuid}", logReq(handleRegisterContainer)).Methods("PUT")
	r.HandleFunc("/pods/{uuid}", logReq(handleUnregisterContainer)).Methods("DELETE")
	r.HandleFunc("/pods/{uuid}/{app:.*}", logReq(handleRegisterApp)).Methods("PUT")

	if err := http.Serve(l, r); err != nil {
		stderr("Error serving registration HTTP: %v", err)
	}
	close(exitCh)
}

func runPublicServer(l net.Listener) {
	r := mux.NewRouter().Headers("Metadata-Flavor", "AppContainer").
		PathPrefix("/acMetadata/v1").Subrouter()

	mr := r.Methods("GET").Subrouter()

	mr.HandleFunc("/pod/annotations/", logReq(containerGet(handleContainerAnnotations)))
	mr.HandleFunc("/pod/annotations/{name}", logReq(containerGet(handleContainerAnnotation)))
	mr.HandleFunc("/pod/manifest", logReq(containerGet(handlePodManifest)))
	mr.HandleFunc("/pod/uuid", logReq(containerGet(handleContainerUUID)))

	mr.HandleFunc("/apps/{app:.*}/annotations/", logReq(appGet(handleAppAnnotations)))
	mr.HandleFunc("/apps/{app:.*}/annotations/{name}", logReq(appGet(handleAppAnnotation)))
	mr.HandleFunc("/apps/{app:.*}/image/manifest", logReq(appGet(handleImageManifest)))
	mr.HandleFunc("/apps/{app:.*}/image/id", logReq(appGet(handleAppID)))

	r.HandleFunc("/pod/hmac/sign", logReq(handleContainerSign)).Methods("POST")
	r.HandleFunc("/pod/hmac/verify", logReq(handleContainerVerify)).Methods("POST")

	if err := http.Serve(l, r); err != nil {
		stderr("Error serving container HTTP: %v", err)
	}
	close(exitCh)
}

func runMetadataService(args []string) (exit int) {
	log.Print("Metadata service starting...")

	unixl, err := unixListener()
	if err != nil {
		stderr(err.Error())
		return 1
	}
	defer unixl.Close()

	tcpl, err := net.ListenTCP("tcp4", &net.TCPAddr{Port: flagListenPort})
	if err != nil {
		stderr("Error listening on port %v: %v", flagListenPort, err)
		return 1
	}
	defer tcpl.Close()

	if err := initCrypto(); err != nil {
		stderr(err.Error())
		return 1
	}

	go runRegistrationServer(unixl)
	go runPublicServer(tcpl)

	log.Print("Metadata service running...")

	signal.Notify(exitCh, syscall.SIGINT, syscall.SIGTERM)
	<-exitCh

	log.Print("Metadata service exiting...")

	return
}
