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

package cas

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/appc/spec/schema/types"
	"github.com/coreos/rocket/pkg/util"
)

const tstprefix = "cas-test"

func TestBlobStore(t *testing.T) {
	dir, err := ioutil.TempDir("", tstprefix)
	if err != nil {
		t.Fatalf("error creating tempdir: %v", err)
	}
	defer os.RemoveAll(dir)
	ds := NewStore(dir)
	for _, valueStr := range []string{
		"I am a manually placed object",
	} {
		ds.stores[blobType].Write(types.NewHashSHA512([]byte(valueStr)).String(), []byte(valueStr))
	}

	ds.Dump(false)
}

func TestDownloading(t *testing.T) {
	dir, err := ioutil.TempDir("", tstprefix)
	if err != nil {
		t.Fatalf("error creating tempdir: %v", err)
	}
	defer os.RemoveAll(dir)

	imj := `{
			"acKind": "ImageManifest",
			"acVersion": "0.1.1",
			"name": "example.com/test01"
		}`

	entries := []*util.ACIEntry{
		// An empty file
		{
			Contents: "hello",
			Header: &tar.Header{
				Name: "rootfs/file01.txt",
				Size: 5,
			},
		},
	}

	aci, err := util.NewACI(dir, imj, entries)
	if err != nil {
		t.Fatalf("error creating test tar: %v", err)
	}

	// Rewind the ACI
	if _, err := aci.Seek(0, 0); err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	body, err := ioutil.ReadAll(aci)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fmt.Printf("body: %s\n", body)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(body)
	}))
	defer ts.Close()

	tests := []struct {
		r    Remote
		body []byte
		hit  bool
	}{
		// The Blob entry isn't used
		{Remote{ts.URL, "", "12", ""}, body, false},
		{Remote{ts.URL, "", "12", ""}, body, true},
	}

	ds := NewStore(dir)

	for _, tt := range tests {
		_, err := ds.stores[remoteType].Read(tt.r.Hash())
		if tt.hit == false && err == nil {
			panic("expected miss got a hit")
		}
		if tt.hit == true && err != nil {
			panic("expected a hit got a miss")
		}
		rj, err := tt.r.Marshal()
		if err != nil {
			panic(err)
		}
		ds.stores[remoteType].Write(tt.r.Hash(), rj)
		_, aciFile, err := tt.r.Download(*ds, nil)
		if err != nil {
			t.Fatalf("error downloading aci: %v", err)
		}
		defer os.Remove(aciFile.Name())

		_, err = tt.r.Store(*ds, aciFile)
		if err != nil {
			panic(err)
		}
	}

	ds.Dump(false)
}

func TestResolveKey(t *testing.T) {
	dir, err := ioutil.TempDir("", tstprefix)
	if err != nil {
		t.Fatalf("error creating tempdir: %v", err)
	}
	defer os.RemoveAll(dir)
	ds := NewStore(dir)

	// Set up store (use key == data for simplicity)
	data := []*bytes.Buffer{
		bytes.NewBufferString("sha512-1234567890"),
		bytes.NewBufferString("sha512-abcdefghi"),
		bytes.NewBufferString("sha512-abcjklmno"),
		bytes.NewBufferString("sha512-abcpqwert"),
	}
	for _, d := range data {
		if err := ds.WriteStream(d.String(), d); err != nil {
			t.Fatalf("error writing to store: %v", err)
		}
	}

	// Full key already - should return short version of the full key
	fkl := "sha512-67147019a5b56f5e2ee01e989a8aa4787f56b8445960be2d8678391cf111009bc0780f31001fd181a2b61507547aee4caa44cda4b8bdb238d0e4ba830069ed2c"
	fks := "sha512-67147019a5b56f5e2ee01e989a8aa4787f56b8445960be2d8678391cf111009b"
	for _, k := range []string{fkl, fks} {
		key, err := ds.ResolveKey(k)
		if key != fks {
			t.Errorf("expected ResolveKey to return unaltered short key, but got %q", key)
		}
		if err != nil {
			t.Errorf("expected err=nil, got %v", err)
		}
	}

	// Unambiguous prefix match
	k, err := ds.ResolveKey("sha512-123")
	if k != "sha512-1234567890" {
		t.Errorf("expected %q, got %q", "sha512-1234567890", k)
	}
	if err != nil {
		t.Errorf("expected err=nil, got %v", err)
	}

	// Ambiguous prefix match
	k, err = ds.ResolveKey("sha512-abc")
	if k != "" {
		t.Errorf("expected %q, got %q", "", k)
	}
	if err == nil {
		t.Errorf("expected non-nil error!")
	}
}
