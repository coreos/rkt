package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/coreos/rocket/cas"
	"github.com/coreos/rocket/pkg/keystore"
	"github.com/coreos/rocket/pkg/keystore/keystoretest"
	"github.com/coreos/rocket/pkg/util"

	"github.com/appc/spec/discovery"
)

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
				Labels: map[string]string{
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
				Labels: map[string]string{
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
				Labels: map[string]string{
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
				Labels: map[string]string{
					"val":  "one",
					"arch": defaultArch,
					"os":   defaultOS,
				},
			},
		},
		// combinations
		{
			"one.two/:three,os=four,foo=five,arch=six",
			&discovery.App{
				Name: "one.two/",
				Labels: map[string]string{
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

func TestFetchImage(t *testing.T) {
	dir, err := ioutil.TempDir("", "fetch-image")
	if err != nil {
		t.Fatalf("error creating tempdir: %v", err)
	}
	defer os.RemoveAll(dir)
	ds := cas.NewStore(dir)
	defer ds.Dump(false)

	ks, ksPath, err := keystore.NewTestKeystore()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	defer os.RemoveAll(ksPath)

	key := keystoretest.KeyMap["example.com/app"]
	if _, err := ks.StoreTrustedKeyPrefix("example.com/app", bytes.NewBufferString(key.ArmoredPublicKey)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	aci, err := util.NewACI("example.com/app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer aci.Close()

	// Rewind the ACI
	if _, err := aci.Seek(0, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sig, err := util.NewDetachedSignature(key.ArmoredPrivateKey, aci)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Rewind the ACI.
	if _, err := aci.Seek(0, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch filepath.Ext(r.URL.Path) {
		case ".aci":
			io.Copy(w, aci)
			return
		case ".sig":
			io.Copy(w, sig)
			return
		}
	}))
	defer ts.Close()
	_, err = fetchImage(fmt.Sprintf("%s/app.aci", ts.URL), ds, ks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFetchImageFromEndpoints(t *testing.T) {
	dir, err := ioutil.TempDir("", "fetch-image")
	if err != nil {
		t.Fatalf("error creating tempdir: %v", err)
	}
	defer os.RemoveAll(dir)
	ds := cas.NewStore(dir)
	defer ds.Dump(false)

	ks, ksPath, err := keystore.NewTestKeystore()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	defer os.RemoveAll(ksPath)

	key := keystoretest.KeyMap["example.com/app"]
	if _, err := ks.StoreTrustedKeyPrefix("example.com/app", bytes.NewBufferString(key.ArmoredPublicKey)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	aci, err := util.NewACI("example.com/app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer aci.Close()

	// Rewind the ACI
	if _, err := aci.Seek(0, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sig, err := util.NewDetachedSignature(key.ArmoredPrivateKey, aci)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Rewind the ACI.
	if _, err := aci.Seek(0, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/existingaci.aci":
			io.Copy(w, aci)
			return
		case "/existingaci.sig":
			io.Copy(w, sig)
			return
		default:
			http.NotFound(w, r)
			return
		}
	}))
	defer ts.Close()
	eps := &discovery.Endpoints{
		ACIEndpoints: []discovery.ACIEndpoint{
			discovery.ACIEndpoint{
				ACI: fmt.Sprintf("%s/notexistingaci.aci", ts.URL),
				Sig: fmt.Sprintf("%s/notexistingaci.sig", ts.URL),
			},
			discovery.ACIEndpoint{
				ACI: fmt.Sprintf("%s/existingaci.aci", ts.URL),
				Sig: fmt.Sprintf("%s/existingaci.sig", ts.URL),
			},
		},
	}
	_, err = fetchImageFromEndpoints(eps, ds, ks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Test missing images
	eps = &discovery.Endpoints{
		ACIEndpoints: []discovery.ACIEndpoint{
			discovery.ACIEndpoint{
				ACI: fmt.Sprintf("%s/notexistingaci.aci", ts.URL),
				Sig: fmt.Sprintf("%s/notexistingaci.sig", ts.URL),
			},
			discovery.ACIEndpoint{
				ACI: fmt.Sprintf("%s/notexistingaci2.aci", ts.URL),
				Sig: fmt.Sprintf("%s/notexistingaci2.sig", ts.URL),
			},
		},
	}
	_, err = fetchImageFromEndpoints(eps, ds, ks)
	if err == nil {
		t.Fatalf("expected error fetching from endpoints")
	}
}

func TestSigURLFromImgURL(t *testing.T) {
	tests := []struct {
		in, out string
	}{
		{
			"http://localhost/aci-latest-linux-amd64.aci",
			"http://localhost/aci-latest-linux-amd64.sig",
		},
	}
	for i, tt := range tests {
		out := sigURLFromImgURL(tt.in)
		if out != tt.out {
			t.Errorf("#%d: got %v, want %v", i, out, tt.out)
		}
	}
}
