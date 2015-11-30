// Copyright 2014 The rkt Authors
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
	"archive/tar"
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/coreos/rkt/pkg/aci"
	"github.com/coreos/rkt/pkg/keystore"
	"github.com/coreos/rkt/pkg/keystore/keystoretest"
	"github.com/coreos/rkt/rkt/config"
	"github.com/coreos/rkt/store"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/discovery"
	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/schema/types"
)

type httpError struct {
	code    int
	message string
}

func (e *httpError) Error() string {
	return fmt.Sprintf("%d: %s", e.code, e.message)
}

type serverHandler struct {
	body []byte
	t    *testing.T
	auth string
}

func getSecFlags(opts []string, defOpts string, fm map[string]int) *secFlags {
	bf, err := newBitFlags(opts, defOpts, fm)
	if err != nil {
		panic(fmt.Sprintf("fetch-test: problem initializing flags: %s", err))
	}

	return (*secFlags)(bf)
}

var (
	insecParams   = "image,tls"
	insecureFlags = getSecFlags(insecureOptions, insecParams, insecureOptionsMap)
	secureFlags   = getSecFlags(insecureOptions, "none", insecureOptionsMap)
)

func (h *serverHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch h.auth {
	case "deny":
		if _, ok := r.Header[http.CanonicalHeaderKey("Authorization")]; ok {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	case "none":
		// no auth to do.
	case "basic":
		payload, httpErr := getAuthPayload(r, "Basic")
		if httpErr != nil {
			w.WriteHeader(httpErr.code)
			return
		}
		creds, err := base64.StdEncoding.DecodeString(string(payload))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		parts := strings.Split(string(creds), ":")
		if len(parts) != 2 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		user := parts[0]
		password := parts[1]
		if user != "bar" || password != "baz" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
	case "bearer":
		payload, httpErr := getAuthPayload(r, "Bearer")
		if httpErr != nil {
			w.WriteHeader(httpErr.code)
			return
		}
		if payload != "sometoken" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
	default:
		panic("bug in test")
	}
	w.Write(h.body)
}

func getAuthPayload(r *http.Request, authType string) (string, *httpError) {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		err := &httpError{
			code:    http.StatusUnauthorized,
			message: "No auth",
		}
		return "", err
	}
	parts := strings.Split(auth, " ")
	if len(parts) != 2 {
		err := &httpError{
			code:    http.StatusBadRequest,
			message: "Malformed auth",
		}
		return "", err
	}
	if parts[0] != authType {
		err := &httpError{
			code:    http.StatusUnauthorized,
			message: "Wrong auth",
		}
		return "", err
	}
	return parts[1], nil
}

type testHeaderer struct {
	h http.Header
}

func (h *testHeaderer) Header() http.Header {
	return h.h
}

func TestNewDiscoveryApp(t *testing.T) {
	tests := []struct {
		in string

		w *discovery.App
	}{
		// not a valid AC name
		{
			"bad AC name",
			nil,
		},
		// simple case - default arch, os should be substituted
		{
			"foo.com/bar",
			&discovery.App{
				Name: "foo.com/bar",
				Labels: map[types.ACIdentifier]string{
					"arch": defaultArch,
					"os":   defaultOS,
				},
			},
		},
		// overriding arch, os should work
		{
			"www.abc.xyz/my/app,os=freebsd,arch=i386",
			&discovery.App{
				Name: "www.abc.xyz/my/app",
				Labels: map[types.ACIdentifier]string{
					"arch": "i386",
					"os":   "freebsd",
				},
			},
		},
		// setting version should work
		{
			"yes.com/no:v1.2.3",
			&discovery.App{
				Name: "yes.com/no",
				Labels: map[types.ACIdentifier]string{
					"version": "v1.2.3",
					"arch":    defaultArch,
					"os":      defaultOS,
				},
			},
		},
		// arbitrary user-supplied labels
		{
			"example.com/foo/haha,val=one",
			&discovery.App{
				Name: "example.com/foo/haha",
				Labels: map[types.ACIdentifier]string{
					"val":  "one",
					"arch": defaultArch,
					"os":   defaultOS,
				},
			},
		},
		// combinations
		{
			"one.two/appname:three,os=four,foo=five,arch=six",
			&discovery.App{
				Name: "one.two/appname",
				Labels: map[types.ACIdentifier]string{
					"version": "three",
					"os":      "four",
					"foo":     "five",
					"arch":    "six",
				},
			},
		},
	}
	for i, tt := range tests {
		g := newDiscoveryApp(tt.in)
		if !reflect.DeepEqual(g, tt.w) {
			t.Errorf("#%d: got %v, want %v", i, g, tt.w)
		}
	}
}

func TestDownloading(t *testing.T) {
	dir, err := ioutil.TempDir("", "download-image")
	if err != nil {
		t.Fatalf("error creating tempdir: %v", err)
	}
	defer os.RemoveAll(dir)

	imj := `{
			"acKind": "ImageManifest",
			"acVersion": "0.7.3",
			"name": "example.com/test01"
		}`

	entries := []*aci.ACIEntry{
		// An empty file
		{
			Contents: "hello",
			Header: &tar.Header{
				Name: "rootfs/file01.txt",
				Size: 5,
			},
		},
	}

	aci, err := aci.NewACI(dir, imj, entries)
	if err != nil {
		t.Fatalf("error creating test tar: %v", err)
	}

	// Rewind the ACI
	if _, err := aci.Seek(0, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	body, err := ioutil.ReadAll(aci)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	noauthServer := &serverHandler{
		body: body,
		t:    t,
		auth: "none",
	}
	basicServer := &serverHandler{
		body: body,
		t:    t,
		auth: "basic",
	}
	oauthServer := &serverHandler{
		body: body,
		t:    t,
		auth: "bearer",
	}
	denyServer := &serverHandler{
		body: body,
		t:    t,
		auth: "deny",
	}
	noAuthTS := httptest.NewTLSServer(noauthServer)
	defer noAuthTS.Close()
	basicTS := httptest.NewTLSServer(basicServer)
	defer basicTS.Close()
	oauthTS := httptest.NewTLSServer(oauthServer)
	defer oauthTS.Close()
	denyAuthTS := httptest.NewServer(denyServer)
	noAuth := http.Header{}
	// YmFyOmJheg== is base64(bar:baz)
	basicAuth := http.Header{"Authorization": {"Basic YmFyOmJheg=="}}
	bearerAuth := http.Header{"Authorization": {"Bearer sometoken"}}
	urlToName := map[string]string{
		noAuthTS.URL:   "no auth",
		basicTS.URL:    "basic",
		oauthTS.URL:    "oauth",
		denyAuthTS.URL: "deny auth",
	}
	tests := []struct {
		ACIURL   string
		hit      bool
		options  http.Header
		authFail bool
	}{
		{noAuthTS.URL, false, noAuth, false},
		{noAuthTS.URL, true, noAuth, false},
		{noAuthTS.URL, true, bearerAuth, false},
		{noAuthTS.URL, true, basicAuth, false},

		{basicTS.URL, false, noAuth, true},
		{basicTS.URL, false, bearerAuth, true},
		{basicTS.URL, false, basicAuth, false},

		{oauthTS.URL, false, noAuth, true},
		{oauthTS.URL, false, basicAuth, true},
		{oauthTS.URL, false, bearerAuth, false},

		{denyAuthTS.URL, false, basicAuth, false},
		{denyAuthTS.URL, true, bearerAuth, false},
		{denyAuthTS.URL, true, noAuth, false},
	}

	s, err := store.NewStore(dir)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	for _, tt := range tests {
		_, ok, err := s.GetRemote(tt.ACIURL)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if tt.hit == false && ok {
			t.Fatalf("expected miss got a hit")
		}
		if tt.hit == true && !ok {
			t.Fatalf("expected a hit got a miss")
		}
		parsed, err := url.Parse(tt.ACIURL)
		if err != nil {
			panic(fmt.Sprintf("Invalid url from test server: %s", tt.ACIURL))
		}
		headers := map[string]config.Headerer{
			parsed.Host: &testHeaderer{tt.options},
		}

		ft := &fetcher{
			imageActionData: imageActionData{
				s:             s,
				headers:       headers,
				insecureFlags: insecureFlags,
			},
		}
		_, aciFile, _, err := ft.fetch("", tt.ACIURL, "", nil, "")
		if err == nil {
			defer os.Remove(aciFile.Name())
		}
		if err != nil && !tt.authFail {
			t.Fatalf("expected download to succeed, it failed: %v (server: %q, headers: `%v`)", err, urlToName[tt.ACIURL], tt.options)
		}
		if err == nil && tt.authFail {
			t.Fatalf("expected download to fail, it succeeded (server: %q, headers: `%v`)", urlToName[tt.ACIURL], tt.options)
		}
		if err != nil {
			continue
		}

		key, err := s.WriteACI(aciFile, false)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		rem := store.NewRemote(tt.ACIURL, "")
		rem.BlobKey = key
		err = s.WriteRemote(rem)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
	}

	s.Dump(false)
}

func TestFetchImage(t *testing.T) {
	dir, err := ioutil.TempDir("", "fetch-image")
	if err != nil {
		t.Fatalf("error creating tempdir: %v", err)
	}
	defer os.RemoveAll(dir)
	s, err := store.NewStore(dir)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	defer s.Dump(false)

	ks, ksPath, err := keystore.NewTestKeystore()
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}
	defer os.RemoveAll(ksPath)

	key := keystoretest.KeyMap["example.com/app"]
	if _, err := ks.StoreTrustedKeyPrefix("example.com/app", bytes.NewBufferString(key.ArmoredPublicKey)); err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	a, err := aci.NewBasicACI(dir, "example.com/app")
	defer a.Close()
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	// Rewind the ACI
	if _, err := a.Seek(0, 0); err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	asc, err := aci.NewDetachedSignature(key.ArmoredPrivateKey, a)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	// Rewind the ACI.
	if _, err := a.Seek(0, 0); err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch filepath.Ext(r.URL.Path) {
		case ".aci":
			io.Copy(w, a)
			return
		case ".asc":
			io.Copy(w, asc)
			return
		default:
			t.Fatalf("unknown extension %v", r.URL.Path)
		}
	}))
	defer ts.Close()
	ft := &fetcher{
		imageActionData: imageActionData{
			s:             s,
			ks:            ks,
			insecureFlags: secureFlags,
		},
	}
	_, err = ft.fetchImage(fmt.Sprintf("%s/app.aci", ts.URL), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetStoreKeyFromApp(t *testing.T) {
	dir, err := ioutil.TempDir("", "fetch-image")
	if err != nil {
		t.Fatalf("error creating tempdir: %v", err)
	}
	defer os.RemoveAll(dir)
	s, err := store.NewStore(dir)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	defer s.Dump(false)

	// Test an aci without os and arch labels
	a, err := aci.NewBasicACI(dir, "example.com/app")
	defer a.Close()
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	// Rewind the ACI
	if _, err := a.Seek(0, 0); err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	_, err = s.WriteACI(a, false)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	_, err = getStoreKeyFromApp(s, "example.com/app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

type redirectingServerHandler struct {
	destServer string
}

func (h *redirectingServerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Location", fmt.Sprintf("%s/%s", h.destServer, r.URL.Path))
	w.WriteHeader(http.StatusTemporaryRedirect)
}

type cachingServerHandler struct {
	aciBody []byte
	ascBody []byte
	etag    string
	maxAge  int
	t       *testing.T
}

func (h *cachingServerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch filepath.Ext(r.URL.Path) {
	case ".aci":
		if h.maxAge > 0 {
			w.Header().Set("Cache-Control", fmt.Sprintf("max-age=%d", h.maxAge))
		}
		if h.etag != "" {
			w.Header().Set("ETag", h.etag)
			if cc := r.Header.Get("If-None-Match"); cc == h.etag {
				w.WriteHeader(http.StatusNotModified)
				return
			}
		}
		w.Write(h.aciBody)
		return
	case ".asc":
		w.Write(h.ascBody)
		return
	}
}

func TestFetchImageCache(t *testing.T) {
	dir, err := ioutil.TempDir("", "fetch-image-cache")
	if err != nil {
		t.Fatalf("error creating tempdir: %v", err)
	}
	defer os.RemoveAll(dir)
	s, err := store.NewStore(dir)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	defer s.Dump(false)

	ks, ksPath, err := keystore.NewTestKeystore()
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}
	defer os.RemoveAll(ksPath)

	key := keystoretest.KeyMap["example.com/app"]
	if _, err := ks.StoreTrustedKeyPrefix("example.com/app", bytes.NewBufferString(key.ArmoredPublicKey)); err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	a, err := aci.NewBasicACI(dir, "example.com/app")
	defer a.Close()
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	// Rewind the ACI
	if _, err := a.Seek(0, 0); err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	asc, err := aci.NewDetachedSignature(key.ArmoredPrivateKey, a)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	// Rewind the ACI
	if _, err := a.Seek(0, 0); err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	aciBody, err := ioutil.ReadAll(a)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ascBody, err := ioutil.ReadAll(asc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	nocacheServer := &cachingServerHandler{
		aciBody: aciBody,
		ascBody: ascBody,
		etag:    "",
		maxAge:  0,
		t:       t,
	}
	etagServer := &cachingServerHandler{
		aciBody: aciBody,
		ascBody: ascBody,
		etag:    "123456789",
		maxAge:  0,
		t:       t,
	}
	maxAgeServer := &cachingServerHandler{
		aciBody: aciBody,
		ascBody: ascBody,
		etag:    "",
		maxAge:  10,
		t:       t,
	}
	etagMaxAgeServer := &cachingServerHandler{
		aciBody: aciBody,
		ascBody: ascBody,
		etag:    "123456789",
		maxAge:  10,
		t:       t,
	}

	nocacheTS := httptest.NewServer(nocacheServer)
	defer nocacheTS.Close()
	etagTS := httptest.NewServer(etagServer)
	defer etagTS.Close()
	maxAgeTS := httptest.NewServer(maxAgeServer)
	defer maxAgeTS.Close()
	etagMaxAgeTS := httptest.NewServer(etagMaxAgeServer)
	defer etagMaxAgeTS.Close()

	type testData struct {
		URL             string
		etag            string
		cacheMaxAge     int
		shouldUseCached bool
	}
	tests := []testData{
		{nocacheTS.URL, "", 0, false},
		{etagTS.URL, "123456789", 0, true},
		{maxAgeTS.URL, "", 10, true},
		{etagMaxAgeTS.URL, "123456789", 10, true},
	}
	testFn := func(tt testData, useRedirect bool) {
		aciURL := fmt.Sprintf("%s/app.aci", tt.URL)
		if useRedirect {
			redirectingTS := httptest.NewServer(&redirectingServerHandler{destServer: tt.URL})
			defer redirectingTS.Close()
			aciURL = fmt.Sprintf("%s/app.aci", redirectingTS.URL)
		}
		ft := &fetcher{
			imageActionData: imageActionData{
				s:             s,
				ks:            ks,
				insecureFlags: secureFlags,
			},
			// Skip local store
			noStore: true,
		}
		_, err = ft.fetchImage(aciURL, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		rem, _, err := s.GetRemote(aciURL)
		if err != nil {
			t.Fatalf("Error getting remote info: %v\n", err)
		}
		if rem.ETag != tt.etag {
			t.Errorf("expected remote to have a ETag header argument")
		}
		if rem.CacheMaxAge != tt.cacheMaxAge {
			t.Errorf("expected max-age header argument to be %q", tt.cacheMaxAge)
		}

		downloadTime := rem.DownloadTime
		_, err = ft.fetchImage(aciURL, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		rem, _, err = s.GetRemote(aciURL)
		if err != nil {
			t.Fatalf("Error getting remote info: %v\n", err)
		}
		if rem.ETag != tt.etag {
			t.Errorf("expected remote to have a ETag header argument")
		}
		if rem.CacheMaxAge != tt.cacheMaxAge {
			t.Errorf("expected max-age header argument to be %q", tt.cacheMaxAge)
		}
		if tt.shouldUseCached {
			if downloadTime != rem.DownloadTime {
				t.Errorf("expected current download time to be the same of the previous one (no download) but they differ")
			}
		} else {
			if downloadTime == rem.DownloadTime {
				t.Errorf("expected current download time to be different from the previous one (new image download) but they are the same")
			}
		}

		if err := s.RemoveACI(rem.BlobKey); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	// repeat the tests with and without a redirecting server
	for i := 0; i <= 1; i++ {
		useRedirect := false
		if i == 1 {
			useRedirect = true
		}
		for _, tt := range tests {
			testFn(tt, useRedirect)
		}
	}
}

func TestSigURLFromImgURL(t *testing.T) {
	tests := []struct {
		in, out string
	}{
		{
			"http://localhost/aci-latest-linux-amd64.aci",
			"http://localhost/aci-latest-linux-amd64.aci.asc",
		},
	}
	for i, tt := range tests {
		out := ascURLFromImgURL(tt.in)
		if out != tt.out {
			t.Errorf("#%d: got %v, want %v", i, out, tt.out)
		}
	}
}

func TestGetMaxAge(t *testing.T) {
	ma := getMaxAge("max-age=10")
	if ma != 10 {
		t.Errorf("got max-age: %d, want %d", ma, 10)
	}
	ma = getMaxAge("no-cache")
	if ma != 0 {
		t.Errorf("got max-age: %d, want %d", ma, 0)
	}
	ma = getMaxAge("no-store")
	if ma != 0 {
		t.Errorf("got max-age: %d, want %d", ma, 0)
	}
}

func TestUseCached(t *testing.T) {
	ma := 10
	t1 := time.Now().Add(-11 * time.Second)
	if useCached(t1, ma) {
		t.Errorf("expected useCached to return false")
	}
	t1 = time.Now().Add(-1 * time.Second)
	if !useCached(t1, ma) {
		t.Errorf("expected useCached to return true")
	}
}
