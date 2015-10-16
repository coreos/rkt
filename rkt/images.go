// Copyright 2015 The rkt Authors
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

//+build linux

package main

import (
	"bytes"
	"container/list"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/coreos/rkt/common/apps"
	"github.com/coreos/rkt/pkg/keystore"
	"github.com/coreos/rkt/rkt/config"
	"github.com/coreos/rkt/stage0"
	"github.com/coreos/rkt/store"
	"github.com/coreos/rkt/version"

	docker2aci "github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/docker2aci/lib"
	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/docker2aci/lib/common"
	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/aci"
	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/discovery"
	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/schema/types"
	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/coreos/ioprogress"
	"github.com/coreos/rkt/Godeps/_workspace/src/golang.org/x/crypto/openpgp"
	pgperrors "github.com/coreos/rkt/Godeps/_workspace/src/golang.org/x/crypto/openpgp/errors"
)

type imageActionData struct {
	s                  *store.Store
	ks                 *keystore.Keystore
	headers            map[string]config.Headerer
	dockerAuth         map[string]config.BasicCredentials
	insecureSkipVerify bool
	debug              bool
}

type finder struct {
	imageActionData
	storeOnly bool
	noStore   bool
	withDeps  bool
}

// findImages uses findImage to attain a list of image hashes
func (f *finder) findImages(al *apps.Apps) error {
	return al.Walk(func(app *apps.App) error {
		h, err := f.findImage(app.Image, app.Asc)
		if err != nil {
			return err
		}
		app.ImageID = *h
		return nil
	})
}

// findImage will recognize a ACI hash and use that or will fetch using the
// provided img (image name string, local file, URL).
func (f *finder) findImage(img string, asc string) (*types.Hash, error) {
	// check if it is a valid hash, if so let it pass through
	h, err := types.NewHash(img)
	if err == nil {
		fullKey, err := f.s.ResolveKey(img)
		if err != nil {
			return nil, fmt.Errorf("could not resolve image ID: %v", err)
		}
		h, err = types.NewHash(fullKey)
		if err != nil {
			// should never happen
			panic(err)
		}
		return h, nil
	}

	// try fetching the image, potentially remotely
	ft := &fetcher{
		imageActionData: f.imageActionData,
		storeOnly:       f.storeOnly,
		noStore:         f.noStore,
		withDeps:        f.withDeps,
	}
	key, err := ft.fetchImage(img, asc)
	if err != nil {
		return nil, err
	}
	h, err = types.NewHash(key)
	if err != nil {
		// should never happen
		panic(err)
	}

	return h, nil
}

var errStatusAccepted = errors.New("server is still processing the request")

type fetcher struct {
	imageActionData
	storeOnly bool
	noStore   bool
	withDeps  bool
}

// fetchImage will take an image as either a URL or a name string and import it
// into the store if found. If asc is not "", it must exist as a local file and
// will be used as the signature file for verification, unless verification is
// disabled. If f.withDeps is true also image dependencies are fetched.
func (f *fetcher) fetchImage(img string, asc string) (string, error) {
	hash, err := f.fetchSingleImage(img, asc)
	if err != nil {
		return "", err
	}
	if f.withDeps {
		err = f.fetchImageDeps(hash)
		if err != nil {
			return "", err
		}
	}
	return hash, nil
}

func (f *fetcher) getImageDeps(hash string) (types.Dependencies, error) {
	key, err := f.s.ResolveKey(hash)
	if err != nil {
		return nil, err
	}
	im, err := f.s.GetImageManifest(key)
	if err != nil {
		return nil, err
	}
	return im.Dependencies, nil
}

func (f *fetcher) addImageDeps(hash string, imgsl *list.List, seen map[string]struct{}) error {
	dependencies, err := f.getImageDeps(hash)
	if err != nil {
		return err
	}
	for _, d := range dependencies {
		app, err := discovery.NewApp(d.ImageName.String(), d.Labels.ToMap())
		if err != nil {
			return err
		}
		imgsl.PushBack(app.String())
		if _, ok := seen[app.String()]; ok {
			return fmt.Errorf("dependency %s specified multiple times in the dependency tree for image ID: %s", app.String(), hash)
		}
		seen[app.String()] = struct{}{}
	}
	return nil
}

// fetchImageDeps will recursively fetch all the image dependencies
func (f *fetcher) fetchImageDeps(hash string) error {
	imgsl := list.New()
	seen := map[string]struct{}{}
	f.addImageDeps(hash, imgsl, seen)
	for el := imgsl.Front(); el != nil; el = el.Next() {
		img := el.Value.(string)
		hash, err := f.fetchSingleImage(img, "")
		if err != nil {
			return err
		}
		f.addImageDeps(hash, imgsl, seen)
	}
	return nil
}

// fetchSingleImage will take an image as either a URL or a name string and
// import it into the store if found. If asc is not "", it must exist as a
// local file and will be used as the signature file for verification, unless
// verification is disabled.
func (f *fetcher) fetchSingleImage(img string, asc string) (string, error) {
	var (
		ascFile *os.File
		err     error
	)
	if asc != "" && f.ks != nil {
		ascFile, err = os.Open(asc)
		if err != nil {
			return "", fmt.Errorf("unable to open signature file: %v", err)
		}
		defer ascFile.Close()
	}

	u, err := url.Parse(img)
	if err != nil {
		return "", fmt.Errorf("not a valid image reference (%s)", img)
	}

	// if img refers to a local file, ensure the scheme is file:// and make the url path absolute
	_, err = os.Stat(u.Path)
	if err == nil {
		u.Path, err = filepath.Abs(u.Path)
		if err != nil {
			return "", fmt.Errorf("unable to get abs path: %v", err)
		}
		u.Scheme = "file"
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("unable to access %q: %v", img, err)
	}

	switch u.Scheme {
	case "":
		// check if os and arch are valid early
		if app := newDiscoveryApp(img); app != nil {
			if err := types.IsValidOSArch(app.Labels, stage0.ValidOSArch); err != nil {
				return "", err
			}
		}
		return f.fetchImageByName(img, ascFile)
	case "http", "https", "file", "docker":
		return f.fetchImageByURL(u, ascFile)
	}

	return "", fmt.Errorf("rkt only supports fetching for image name and http, https, docker or file URLs (%s)", img)
}

// fetchImageByName will try to fetch an image using an image name string.
func (f *fetcher) fetchImageByName(img string, ascFile *os.File) (string, error) {
	// check the store
	if !f.noStore {
		key, err := getStoreKeyFromApp(f.s, img)
		if err == nil {
			stderr("rkt: using image from local store for image name %s", img)
			return key, nil
		}
		switch err.(type) {
		// ignore the error if it's a store.ACINotFoundError
		case store.ACINotFoundError:
		default:
			return "", err
		}
	}

	// do remote fetching
	if !f.storeOnly {
		// Do image discovery
		app := newDiscoveryApp(img)
		if app == nil {
			return "", fmt.Errorf("invalid image name for discovery: %s", img)
		}
		stderr("rkt: searching for app image %s", img)
		ep, err := discoverApp(app, true)
		if err != nil {
			return "", fmt.Errorf("discovery failed for %q: %v", img, err)
		}
		latest := false
		// No specified version label, mark it as latest
		if _, ok := app.Labels["version"]; !ok {
			latest = true
		}
		return f.fetchImageFromEndpoints(app.Name.String(), ep, ascFile, latest)
	}

	return "", fmt.Errorf("unable to fetch image for image name: %s", img)
}

// fetchImageByURL will try fetch an image using an URL.
func (f *fetcher) fetchImageByURL(u *url.URL, ascFile *os.File) (string, error) {
	// Always fetch if it's a file
	if u.Scheme == "file" {
		stderr("rkt: using image from file %s", u.Path)
		return f.fetchImageFromURL(u.String(), u.Scheme, ascFile, false)
	}

	// check the store
	if !f.noStore {
		rem, ok, err := f.s.GetRemote(u.String())
		if err != nil {
			return "", err
		}
		if ok {
			stderr("rkt: using image from local store for url %s", u.String())
			return rem.BlobKey, nil
		}
	}

	// do remote fetching
	if !f.storeOnly {
		latest := false
		if u.Scheme == "docker" {
			dockerURL := common.ParseDockerURL(path.Join(u.Host, u.Path))
			if dockerURL.Tag == "latest" {
				latest = true
			}
		}
		stderr("rkt: remote fetching from url %s", u.String())
		return f.fetchImageFromURL(u.String(), u.Scheme, ascFile, latest)
	}

	return "", fmt.Errorf("unable to fetch image for url %s", u.String())
}

func (f *fetcher) fetchImageFromEndpoints(appName string, ep *discovery.Endpoints, ascFile *os.File, latest bool) (string, error) {
	stderr("rkt: remote fetching from url %s", ep.ACIEndpoints[0].ACI)
	return f.fetchImageFrom(appName, ep.ACIEndpoints[0].ACI, ep.ACIEndpoints[0].ASC, "", ascFile, latest)
}

func (f *fetcher) fetchImageFromURL(imgurl string, scheme string, ascFile *os.File, latest bool) (string, error) {
	return f.fetchImageFrom("", imgurl, ascURLFromImgURL(imgurl), scheme, ascFile, latest)
}

// fetchImageFrom fetches an image from the aciURL.
func (f *fetcher) fetchImageFrom(appName string, aciURL, ascURL, scheme string, ascFile *os.File, latest bool) (string, error) {
	var rem *store.Remote

	if f.insecureSkipVerify && f.ks != nil {
		stderr("rkt: warning: TLS verification and signature verification has been disabled")
	}
	if !f.insecureSkipVerify && scheme == "docker" {
		return "", fmt.Errorf("signature verification for docker images is not supported (try --insecure-skip-verify)")
	}

	if scheme != "file" {
		var err error
		ok := false
		rem, ok, err = f.s.GetRemote(aciURL)
		if err != nil {
			return "", err
		}
		if ok && useCached(rem.DownloadTime, rem.CacheMaxAge) {
			stderr("rkt: using cached image from local store")
			return rem.BlobKey, nil
		}
	}

	if scheme != "file" && f.debug {
		stderr("rkt: fetching image from %s", aciURL)
	}

	var etag string
	if rem != nil {
		etag = rem.ETag
	}
	entity, aciFile, cd, err := f.fetch(appName, aciURL, ascURL, ascFile, etag)
	if err != nil {
		return "", err
	}
	if cd != nil && cd.useCached {
		if rem != nil {
			return rem.BlobKey, nil
		} else {
			// should never happen
			panic("asked to use cached image but remote is nil")
		}
	}
	if scheme != "file" {
		defer os.Remove(aciFile.Name())
	}

	if entity != nil && !f.insecureSkipVerify {
		stderr("rkt: signature verified:")
		for _, v := range entity.Identities {
			stderr("  %s", v.Name)
		}
	}
	key, err := f.s.WriteACI(aciFile, latest)
	if err != nil {
		return "", err
	}

	if scheme != "file" {
		rem := store.NewRemote(aciURL, ascURL)
		rem.BlobKey = key
		rem.DownloadTime = time.Now()
		if cd != nil {
			rem.ETag = cd.etag
			rem.CacheMaxAge = cd.maxAge
		}
		err = f.s.WriteRemote(rem)
		if err != nil {
			return "", err
		}
	}

	return key, nil
}

// fetch opens/downloads and verifies the remote ACI.
// If appName is not "", it will be used to check that the manifest contain the correct appName
// If ascFile is not nil, it will be used as the signature file and ascURL will be ignored.
// If Keystore is nil signature verification will be skipped, regardless of ascFile.
// fetch returns the signer, an *os.File representing the ACI, and an error if any.
// err will be nil if the ACI fetches successfully and the ACI is verified.
func (f *fetcher) fetch(appName string, aciURL, ascURL string, ascFile *os.File, etag string) (*openpgp.Entity, *os.File, *cacheData, error) {
	var (
		entity *openpgp.Entity
		cd     *cacheData
	)

	u, err := url.Parse(aciURL)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("error parsing ACI url: %v", err)
	}
	if u.Scheme == "docker" {
		registryURL := strings.TrimPrefix(aciURL, "docker://")

		storeTmpDir, err := f.s.TmpDir()
		if err != nil {
			return nil, nil, nil, fmt.Errorf("error creating temporary dir for docker to ACI conversion: %v", err)
		}
		tmpDir, err := ioutil.TempDir(storeTmpDir, "docker2aci-")
		if err != nil {
			return nil, nil, nil, err
		}
		defer os.RemoveAll(tmpDir)

		indexName := docker2aci.GetIndexName(registryURL)
		user := ""
		password := ""
		if creds, ok := f.dockerAuth[indexName]; ok {
			user = creds.User
			password = creds.Password
		}
		acis, err := docker2aci.Convert(registryURL, true, tmpDir, tmpDir, user, password)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("error converting docker image to ACI: %v", err)
		}

		aciFile, err := os.Open(acis[0])
		if err != nil {
			return nil, nil, nil, fmt.Errorf("error opening squashed ACI file: %v", err)
		}

		return nil, aciFile, nil, nil
	}

	// attempt to automatically fetch the public key in case it is available on a TLS connection.
	if globalFlags.TrustKeysFromHttps && !globalFlags.InsecureSkipVerify && appName != "" {
		pkls, err := getPubKeyLocations(appName, false, globalFlags.Debug)
		if err != nil {
			stderr("Error determining key location: %v", err)
		} else {
			// no http, don't ask user for accepting the key, no overriding
			if err := addKeys(pkls, appName, false, true, false); err != nil {
				stderr("Error adding keys: %v", err)
			}
		}
	}

	var retrySignature bool
	if f.ks != nil && ascFile == nil {
		u, err := url.Parse(ascURL)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("error parsing ASC url: %v", err)
		}
		if u.Scheme == "file" {
			ascFile, err = os.Open(u.Path)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("error opening signature file: %v", err)
			}
		} else {
			stderr("Downloading signature from %v\n", ascURL)
			ascFile, err = f.s.TmpFile()
			if err != nil {
				return nil, nil, nil, fmt.Errorf("error setting up temporary file: %v", err)
			}
			defer os.Remove(ascFile.Name())

			err = f.downloadSignatureFile(ascURL, ascFile)
			switch err {
			case errStatusAccepted:
				retrySignature = true
				stderr("rkt: server requested deferring the signature download")
			case nil:
				break
			default:
				return nil, nil, nil, fmt.Errorf("error downloading the signature file: %v", err)
			}
		}
		defer ascFile.Close()
	}

	// check if the identity used by the signature is in the store before a
	// possibly expensive download. This is only an optimization and it's
	// ok to skip the test if the signature will be downloaded later.
	if !retrySignature && f.ks != nil && appName != "" {
		if _, err := ascFile.Seek(0, 0); err != nil {
			return nil, nil, nil, fmt.Errorf("error seeking signature file: %v", err)
		}
		if entity, err = f.ks.CheckSignature(appName, bytes.NewReader([]byte{}), ascFile); err != nil {
			if _, ok := err.(pgperrors.SignatureError); !ok {
				return nil, nil, nil, err
			}
		}
	}

	var aciFile *os.File
	if u.Scheme == "file" {
		aciFile, err = os.Open(u.Path)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("error opening ACI file: %v", err)
		}
	} else {
		aciFile, err = f.s.TmpFile()
		if err != nil {
			return nil, aciFile, nil, fmt.Errorf("error setting up temporary file: %v", err)
		}
		defer os.Remove(aciFile.Name())

		if cd, err = f.downloadACI(aciURL, aciFile, etag); err != nil {
			return nil, nil, nil, fmt.Errorf("error downloading ACI: %v", err)
		}
		if cd.useCached {
			return nil, nil, cd, nil
		}
	}

	if retrySignature {
		if err = f.downloadSignatureFile(ascURL, ascFile); err != nil {
			return nil, aciFile, nil, fmt.Errorf("error downloading the signature file: %v", err)
		}
	}

	manifest, err := aci.ManifestFromImage(aciFile)
	if err != nil {
		return nil, aciFile, nil, err
	}
	// Check if the downloaded ACI has the correct app name.
	// The check is only performed when the aci is downloaded through the
	// discovery protocol, but not with local files or full URL.
	if appName != "" && manifest.Name.String() != appName {
		return nil, aciFile, nil,
			fmt.Errorf("error when reading the app name: %q expected but %q found",
				appName, manifest.Name.String())
	}

	if f.ks != nil {
		if _, err := aciFile.Seek(0, 0); err != nil {
			return nil, aciFile, nil, fmt.Errorf("error seeking ACI file: %v", err)
		}
		if _, err := ascFile.Seek(0, 0); err != nil {
			return nil, aciFile, nil, fmt.Errorf("error seeking signature file: %v", err)
		}
		if entity, err = f.ks.CheckSignature(manifest.Name.String(), aciFile, ascFile); err != nil {
			return nil, aciFile, nil, err
		}
	}

	if _, err := aciFile.Seek(0, 0); err != nil {
		return nil, aciFile, nil, fmt.Errorf("error seeking ACI file: %v", err)
	}
	return entity, aciFile, cd, nil
}

type writeSyncer interface {
	io.Writer
	Sync() error
}

// downloadACI gets the aci specified at aciurl
func (f *fetcher) downloadACI(aciurl string, out writeSyncer, etag string) (*cacheData, error) {
	return f.downloadHTTP(aciurl, "ACI", out, etag)
}

// downloadSignatureFile gets the signature specified at sigurl
func (f *fetcher) downloadSignatureFile(sigurl string, out writeSyncer) error {
	_, err := f.downloadHTTP(sigurl, "signature", out, "")
	return err

}

// downloadHTTP retrieves url, creating a temp file using getTempFile
// http:// and https:// urls supported
func (f *fetcher) downloadHTTP(url, label string, out writeSyncer, etag string) (*cacheData, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	transport := http.DefaultTransport
	if f.insecureSkipVerify {
		transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}

	client := &http.Client{Transport: transport}
	f.setHTTPHeaders(req, etag)

	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fmt.Errorf("too many redirects")
		}
		f.setHTTPHeaders(req, etag)
		return nil
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	cd := &cacheData{}
	// TODO(jonboulle): handle http more robustly (redirects?)
	switch res.StatusCode {
	case http.StatusAccepted:
		// If the server returns Status Accepted (HTTP 202), we should retry
		// downloading the signature later.
		return nil, errStatusAccepted
	case http.StatusOK:
		fallthrough
	case http.StatusNotModified:
		cd.etag = res.Header.Get("ETag")
		cd.maxAge = getMaxAge(res.Header.Get("Cache-Control"))
		cd.useCached = (res.StatusCode == http.StatusNotModified)
		if cd.useCached {
			return cd, nil
		}
	default:
		return nil, fmt.Errorf("bad HTTP status code: %d", res.StatusCode)
	}

	prefix := "Downloading " + label
	fmtBytesSize := 18
	barSize := int64(80 - len(prefix) - fmtBytesSize)
	bar := ioprogress.DrawTextFormatBarForW(barSize, os.Stderr)
	fmtfunc := func(progress, total int64) string {
		// Content-Length is set to -1 when unknown.
		if total == -1 {
			return fmt.Sprintf(
				"%s: %v of an unknown total size",
				prefix,
				ioprogress.ByteUnitStr(progress),
			)
		}
		return fmt.Sprintf(
			"%s: %s %s",
			prefix,
			bar(progress, total),
			ioprogress.DrawTextFormatBytes(progress, total),
		)
	}

	reader := &ioprogress.Reader{
		Reader:       res.Body,
		Size:         res.ContentLength,
		DrawFunc:     ioprogress.DrawTerminalf(os.Stderr, fmtfunc),
		DrawInterval: time.Second,
	}

	if _, err := io.Copy(out, reader); err != nil {
		return nil, fmt.Errorf("error copying %s: %v", label, err)
	}

	if err := out.Sync(); err != nil {
		return nil, fmt.Errorf("error writing %s: %v", label, err)
	}

	return cd, nil
}

func (f *fetcher) setHTTPHeaders(req *http.Request, etag string) {
	options := make(http.Header)
	// Send credentials only over secure channel
	if req.URL.Scheme == "https" {
		if hostOpts, ok := f.headers[req.URL.Host]; ok {
			options = hostOpts.Header()
		}
	}
	for k, v := range options {
		for _, e := range v {
			req.Header.Add(k, e)
		}
	}
	if etag != "" {
		req.Header.Add("If-None-Match", etag)
	}
	req.Header.Add("User-Agent", fmt.Sprintf("rkt/%s", version.Version))
}

func ascURLFromImgURL(imgurl string) string {
	s := strings.TrimSuffix(imgurl, ".aci")
	return s + ".aci.asc"
}

// newDiscoveryApp creates a discovery app if the given img is an app name and
// has a URL-like structure, for example example.com/reduce-worker.
// Or it returns nil.
func newDiscoveryApp(img string) *discovery.App {
	app, err := discovery.NewAppFromString(img)
	if err != nil {
		return nil
	}
	u, err := url.Parse(app.Name.String())
	if err != nil || u.Scheme != "" {
		return nil
	}
	if _, ok := app.Labels["arch"]; !ok {
		app.Labels["arch"] = defaultArch
	}
	if _, ok := app.Labels["os"]; !ok {
		app.Labels["os"] = defaultOS
	}
	return app
}

func discoverApp(app *discovery.App, insecure bool) (*discovery.Endpoints, error) {
	ep, attempts, err := discovery.DiscoverEndpoints(*app, insecure)
	if globalFlags.Debug {
		for _, a := range attempts {
			stderr("meta tag 'ac-discovery' not found on %s: %v", a.Prefix, a.Error)
		}
	}
	if err != nil {
		return nil, err
	}
	if len(ep.ACIEndpoints) == 0 {
		return nil, fmt.Errorf("no endpoints discovered")
	}
	return ep, nil
}

func getStoreKeyFromApp(s *store.Store, img string) (string, error) {
	app, err := discovery.NewAppFromString(img)
	if err != nil {
		return "", fmt.Errorf("cannot parse the image name: %v", err)
	}
	labels, err := types.LabelsFromMap(app.Labels)
	if err != nil {
		return "", fmt.Errorf("invalid labels in the name: %v", err)
	}
	key, err := s.GetACI(app.Name, labels)
	if err != nil {
		switch err.(type) {
		case store.ACINotFoundError:
			return "", err
		default:
			return "", fmt.Errorf("cannot find image: %v", err)
		}
	}
	return key, nil
}

func getStoreKeyFromAppOrHash(s *store.Store, input string) (string, error) {
	var key string
	if _, err := types.NewHash(input); err == nil {
		key, err = s.ResolveKey(input)
		if err != nil {
			return "", fmt.Errorf("cannot resolve image ID: %v", err)
		}
	} else {
		key, err = getStoreKeyFromApp(s, input)
		if err != nil {
			return "", fmt.Errorf("cannot find image: %v", err)
		}
	}
	return key, nil
}

type cacheData struct {
	useCached bool
	etag      string
	maxAge    int
}

func getMaxAge(headerValue string) int {
	var MaxAge int = 0

	if len(headerValue) > 0 {
		parts := strings.Split(headerValue, " ")
		for i := 0; i < len(parts); i++ {
			attr, val := parts[i], ""
			if j := strings.Index(attr, "="); j >= 0 {
				attr, val = attr[:j], attr[j+1:]
			}
			lowerAttr := strings.ToLower(attr)

			switch lowerAttr {
			case "no-store":
				MaxAge = 0
				continue
			case "no-cache":
				MaxAge = 0
				continue
			case "max-age":
				secs, err := strconv.Atoi(val)
				if err != nil || secs != 0 && val[0] == '0' {
					break
				}
				if secs <= 0 {
					MaxAge = 0
				} else {
					MaxAge = secs
				}
				continue
			}
		}
	}
	return MaxAge
}

// useCached checks if downloadTime plus maxAge is before/after the current time.
// return true if the cached image should be used, false otherwise.
func useCached(downloadTime time.Time, maxAge int) bool {
	freshnessLifetime := int(time.Now().Sub(downloadTime).Seconds())
	if maxAge > 0 && freshnessLifetime < maxAge {
		return true
	}
	return false
}
