package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/coreos/rocket/Godeps/_workspace/src/github.com/gorilla/mux"

	"github.com/coreos/rocket/app-container/schema"
	"github.com/coreos/rocket/app-container/schema/types"
	"github.com/coreos/rocket/metadata"
)

var (
	cmdMetadataSvc = &Command{
		Name:    "metadatasvc",
		Summary: "Run metadata service",
		Usage:   "[--src-addr CIDR] [--listen-port PORT] [--no-idle]",
		Run:     runMetadataSvc,
	}
)

type container struct {
	manifest schema.ContainerRuntimeManifest
	apps     map[string]*schema.AppManifest
	ip       string
}

var (
	containerByIP  = make(map[string]*container)
	containerByUID = make(map[types.UUID]*container)
	hmacKey       [sha256.Size]byte

	listenPort    uint
	srcAddrs      string
	noIdle        bool

	exitCh        chan bool
)

const (
	listenFdsStart = 3
)

func init() {
	cmdMetadataSvc.Flags.StringVar(&srcAddrs, "src-addr", "0.0.0.0/0", "source address/range for iptables")
	cmdMetadataSvc.Flags.UintVar(&listenPort, "listen-port", metadata.MetadataSvcPrvPort, "listen port")
	cmdMetadataSvc.Flags.BoolVar(&noIdle, "no-idle", false, "exit when last container is unregistered")
}

func modifyIPTables(cmd, dstPort string) error {
	args := []string{"-t", "nat", cmd, "PREROUTING",
		"-p", "tcp", "-s", srcAddrs, "-d", metadata.MetadataSvcIP, "--dport", strconv.Itoa(metadata.MetadataSvcPubPort),
		"-j", "REDIRECT", "--to-port", dstPort}

	return exec.Command("iptables", args...).Run()
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

	remoteIP := strings.Split(r.RemoteAddr, ":")[0]
	if _, ok := containerByIP[remoteIP]; ok {
		// not allowed from container IP
		w.WriteHeader(http.StatusForbidden)
		return
	}

	containerIP := queryValue(r.URL, "ip")
	if containerIP == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "ip missing")
		return
	}

	c := &container{
		apps: make(map[string]*schema.AppManifest),
		ip:   containerIP,
	}

	if err := json.NewDecoder(r.Body).Decode(&c.manifest); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "JSON-decoding failed: %v", err)
		return
	}

	containerByIP[containerIP] = c
	containerByUID[c.manifest.UUID] = c

	w.WriteHeader(http.StatusOK)
}

func handleUnregisterContainer(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	uid, err := types.NewUUID(mux.Vars(r)["uid"])
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "UUID is missing or malformed: %v", err)
		return
	}

	c, ok := containerByUID[*uid]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, "Container with given UUID not found")
		return
	}

	delete(containerByUID, *uid)
	delete(containerByIP, c.ip)
	w.WriteHeader(http.StatusOK)

	if noIdle && len(containerByUID) == 0 {
		exitCh <-true
	}
}

func handleRegisterApp(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	remoteIP := strings.Split(r.RemoteAddr, ":")[0]
	if _, ok := containerByIP[remoteIP]; ok {
		// not allowed from container IP
		w.WriteHeader(http.StatusForbidden)
		return
	}

	uid, err := types.NewUUID(mux.Vars(r)["uid"])
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "UUID is missing or mulformed: %v", err)
		return
	}

	c, ok := containerByUID[*uid]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, "Container with given UUID not found")
		return
	}

	an := mux.Vars(r)["app"]

	app := &schema.AppManifest{}
	if err := json.NewDecoder(r.Body).Decode(&app); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "JSON-decoding failed: %v", err)
		return
	}

	c.apps[an] = app

	w.WriteHeader(http.StatusOK)
}

func containerGet(h func(w http.ResponseWriter, r *http.Request, c *container)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		remoteIP := strings.Split(r.RemoteAddr, ":")[0]
		c, ok := containerByIP[remoteIP]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, "container by remoteIP (%v) not found", remoteIP)
			return
		}

		h(w, r, c)
	}
}

func appGet(h func(w http.ResponseWriter, r *http.Request, c *container, am *schema.AppManifest)) func(http.ResponseWriter, *http.Request) {
	return containerGet(func(w http.ResponseWriter, r *http.Request, c *container) {
		appname := mux.Vars(r)["app"]

		if am, ok := c.apps[appname]; ok {
			h(w, r, c, am)
		} else {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, "App (%v) not found", appname)
		}
	})
}

func handleContainerAnnotations(w http.ResponseWriter, r *http.Request, c *container) {
	defer r.Body.Close()

	w.Header().Add("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)

	for k, _ := range c.manifest.Annotations {
		fmt.Fprintln(w, k)
	}
}

func handleContainerAnnotation(w http.ResponseWriter, r *http.Request, c *container) {
	defer r.Body.Close()

	k, err := types.NewACName(mux.Vars(r)["name"])
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "Container annotation is not a valid AC Label")
		return
	}

	v, ok := c.manifest.Annotations[*k]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "Container annotation (%v) not found", k)
		return
	}

	w.Header().Add("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(v))
}

func handleContainerManifest(w http.ResponseWriter, r *http.Request, c *container) {
	defer r.Body.Close()

	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(c.manifest); err != nil {
		fmt.Println(err)
	}
}

func handleContainerUID(w http.ResponseWriter, r *http.Request, c *container) {
	defer r.Body.Close()

	uid := c.manifest.UUID.String()

	w.Header().Add("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(uid))
}

func mergeAppAnnotations(am *schema.AppManifest, cm *schema.ContainerRuntimeManifest) types.Annotations {
	merged := make(types.Annotations)

	for k, v := range am.Annotations {
		merged[k] = v
	}

	if app := cm.Apps.Get(am.Name); app != nil {
		for k, v := range app.Annotations {
			merged[k] = v
		}
	}

	return merged
}

func handleAppAnnotations(w http.ResponseWriter, r *http.Request, c *container, am *schema.AppManifest) {
	defer r.Body.Close()

	w.Header().Add("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)

	for k, _ := range mergeAppAnnotations(am, &c.manifest) {
		fmt.Fprintln(w, k)
	}
}

func handleAppAnnotation(w http.ResponseWriter, r *http.Request, c *container, am *schema.AppManifest) {
	defer r.Body.Close()

	k, err := types.NewACName(mux.Vars(r)["name"])
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "App annotation is not a valid AC Label")
		return
	}

	merged := mergeAppAnnotations(am, &c.manifest)

	v, ok := merged[*k]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "App annotation (%v) not found", k)
		return
	}

	w.Header().Add("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(v))
}

func handleAppManifest(w http.ResponseWriter, r *http.Request, c *container, am *schema.AppManifest) {
	defer r.Body.Close()

	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(*am); err != nil {
		fmt.Println(err)
	}
}

func handleAppID(w http.ResponseWriter, r *http.Request, c *container, am *schema.AppManifest) {
	defer r.Body.Close()

	w.Header().Add("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	a := c.manifest.Apps.Get(am.Name)
	if a == nil {
		panic("could not find app in manifest!")
	}
	w.Write([]byte(a.ImageID.String()))
}

func initCrypto() error {
	if n, err := rand.Reader.Read(hmacKey[:]); err != nil || n != len(hmacKey) {
		return fmt.Errorf("failed to generate HMAC Key")
	}
	return nil
}

func digest(r io.Reader) ([]byte, error) {
	digest := sha256.New()
	if _, err := io.Copy(digest, r); err != nil {
		return nil, err
	}
	return digest.Sum(nil), nil
}

func handleContainerSign(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	remoteIP := strings.Split(r.RemoteAddr, ":")[0]
	c, ok := containerByIP[remoteIP]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "Metadata by remoteIP (%v) not found", remoteIP)
		return
	}

	// compute message digest
	d, err := digest(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Digest computation failed: %v", err)
		return
	}

	// HMAC(UID:digest)
	h := hmac.New(sha256.New, hmacKey[:])
	h.Write(c.manifest.UUID[:])
	h.Write(d)

	// Send back digest:HMAC as the signature
	w.Header().Add("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	enc := base64.NewEncoder(base64.StdEncoding, w)
	enc.Write(d)
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

	sig, err := base64.StdEncoding.DecodeString(r.FormValue("signature"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "signature field missing or corrupt: %v", err)
		return
	}

	digest := sig[:sha256.Size]
	sum := sig[sha256.Size:]

	h := hmac.New(sha256.New, hmacKey[:])
	h.Write(uid[:])
	h.Write(digest)

	if hmac.Equal(sum, h.Sum(nil)) {
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

func logReq(h func(w http.ResponseWriter, r *http.Request)) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		resp := &httpResp{w, 0}
		h(resp, r)
		fmt.Printf("%v %v - %v\n", r.Method, r.RequestURI, resp.status)
	}
}

func makeHandlers() http.Handler {
	r := mux.NewRouter()
	r.HandleFunc("/containers/", logReq(handleRegisterContainer)).Methods("POST")
	r.HandleFunc("/containers/{uid}", logReq(handleUnregisterContainer)).Methods("DELETE")
	r.HandleFunc("/containers/{uid}/{app:.*}", logReq(handleRegisterApp)).Methods("PUT")

	acRtr := r.Headers("Metadata-Flavor", "AppContainer header").
		PathPrefix("/acMetadata/v1").Subrouter()

	mr := acRtr.Methods("GET").Subrouter()

	mr.HandleFunc("/container/annotations/", logReq(containerGet(handleContainerAnnotations)))
	mr.HandleFunc("/container/annotations/{name}", logReq(containerGet(handleContainerAnnotation)))
	mr.HandleFunc("/container/manifest", logReq(containerGet(handleContainerManifest)))
	mr.HandleFunc("/container/uid", logReq(containerGet(handleContainerUID)))

	mr.HandleFunc("/apps/{app:.*}/annotations/", logReq(appGet(handleAppAnnotations)))
	mr.HandleFunc("/apps/{app:.*}/annotations/{name}", logReq(appGet(handleAppAnnotation)))
	mr.HandleFunc("/apps/{app:.*}/image/manifest", logReq(appGet(handleAppManifest)))
	mr.HandleFunc("/apps/{app:.*}/image/id", logReq(appGet(handleAppID)))

	acRtr.HandleFunc("/container/hmac/sign", logReq(handleContainerSign)).Methods("POST")
	acRtr.HandleFunc("/container/hmac/verify", logReq(handleContainerVerify)).Methods("POST")

	return r
}

func getListener() (net.Listener, error) {
	s := os.Getenv("LISTEN_FDS")
	if s != "" {
		// socket activated
		lfds, err := strconv.ParseInt(s, 10, 16)
		if err != nil {
			return nil, fmt.Errorf("Error parsing LISTEN_FDS env var: ", err)
		}
		if lfds < 1 {
			return nil, fmt.Errorf("LISTEN_FDS < 1")
		}

		return net.FileListener(os.NewFile(uintptr(listenFdsStart), "listen"))
	} else {
		return net.Listen("tcp4", fmt.Sprintf(":%v", listenPort))
	}
}

func cleanup(port string) {
	if err := modifyIPTables("-D", port); err != nil {
		fmt.Fprintf(os.Stdout, "Error cleaning up iptables: %v\n", err)
	}
}

func runMetadataSvc(args []string) (exit int) {
	l, err := getListener()
	if err != nil {
		fmt.Fprintf(os.Stdout, "Error getting listener: %v\n", err)
		return
	}

	initCrypto()

	port := strings.Split(l.Addr().String(), ":")[1]

	if noIdle {
		//TODO(eyakubovich): this is very racy
		// It's possible for last container to get unregistered
		// and svc gets flagged to shutdown. Then another container
		// starts to launch, sees that port is in use and doesn't
		// start metadata svc only for this one to exit a moment later
		exitCh = make(chan bool, 1)
		// wait for signal and exit
		go func() {
			<-exitCh
			cleanup(port)
			os.Exit(0)
		}()
	}

	if err := modifyIPTables("-A", port); err != nil {
		fmt.Fprintf(os.Stdout, "Error setting up iptables: %v\n", err)
		return 1
	}

	srv := http.Server{
		Handler: makeHandlers(),
	}

	if err = srv.Serve(l); err != nil {
		fmt.Fprintf(os.Stdout, "Error serving HTTP: %v\n", err)
		exit = 1
	}

	cleanup(port)

	return
}
