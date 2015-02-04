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
	"encoding/hex"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/appc/spec/schema/types"
	"github.com/coreos/rocket/pkg/aci"
)

const tstprefix = "cas-test"

func TestBlobStore(t *testing.T) {
	dir, err := ioutil.TempDir("", tstprefix)
	if err != nil {
		t.Fatalf("error creating tempdir: %v", err)
	}
	defer os.RemoveAll(dir)
	ds, err := NewStore(dir)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
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
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(body)
	}))
	defer ts.Close()

	tests := []struct {
		ACIURL string
		SigURL string
		body   []byte
		hit    bool
	}{
		// The Blob entry isn't used
		{ts.URL, "", body, false},
		{ts.URL, "", body, true},
	}

	ds, err := NewStore(dir)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	for _, tt := range tests {
		_, ok, err := ds.GetRemote(tt.ACIURL, tt.SigURL)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if tt.hit == false && ok {
			panic("expected miss got a hit")
		}
		if tt.hit == true && !ok {
			panic("expected a hit got a miss")
		}
		rem := NewRemote(tt.ACIURL, tt.SigURL)
		err = ds.WriteRemote(rem)
		_, aciFile, err := rem.Download(*ds, nil)
		if err != nil {
			t.Fatalf("error downloading aci: %v", err)
		}
		defer os.Remove(aciFile.Name())

		_, err = rem.Store(*ds, aciFile, false)
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
	ds, err := NewStore(dir)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Return a hash key buffer from a hex string
	str2key := func(s string) *bytes.Buffer {
		k, _ := hex.DecodeString(s)
		return bytes.NewBufferString(keyToString(k))
	}

	// Set up store (use key == data for simplicity)
	data := []*bytes.Buffer{
		str2key("12345678900000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"),
		str2key("abcdefabc00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"),
		str2key("abcabcabc00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"),
		str2key("abc01234500000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"),
		str2key("67147019a5b56f5e2ee01e989a8aa4787f56b8445960be2d8678391cf111009bc0780f31001fd181a2b61507547aee4caa44cda4b8bdb238d0e4ba830069ed2c"),
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
	if k != "sha512-1234567890000000000000000000000000000000000000000000000000000000" {
		t.Errorf("expected %q, got %q", "sha512-1234567890000000000000000000000000000000000000000000000000000000", k)
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

func TestGetImageManifest(t *testing.T) {
	dir, err := ioutil.TempDir("", tstprefix)
	if err != nil {
		t.Fatalf("error creating tempdir: %v", err)
	}
	defer os.RemoveAll(dir)
	ds, err := NewStore(dir)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	imj := `{
			"acKind": "ImageManifest",
			"acVersion": "0.1.1",
			"name": "example.com/test01"
		}`

	aci, err := aci.NewACI(dir, imj, nil)
	if err != nil {
		t.Fatalf("error creating test tar: %v", err)
	}
	// Rewind the ACI
	if _, err := aci.Seek(0, 0); err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	key, err := ds.WriteACI(aci, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wanted := "example.com/test01"
	im, err := ds.GetImageManifest(key)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if im.Name.String() != wanted {
		t.Errorf("expected im with name: %s, got: %s", wanted, im.Name.String())
	}

	// test unexistent key
	im, err = ds.GetImageManifest("sha512-aaaaaaaaaaaaaaaaa")
	if err == nil {
		t.Fatalf("expected non-nil error!")
	}
}

// Test an image with 1 dep. The parent provides a dir not provided by the image.
func TestGetAci(t *testing.T) {
	type test struct {
		name     types.ACName
		labels   types.Labels
		expected int // the aci index to expect or -1 if not result expected,
	}

	type acidef struct {
		imj    string
		latest bool
	}

	dir, err := ioutil.TempDir("", tstprefix)
	if err != nil {
		t.Fatalf("error creating tempdir: %v", err)
	}
	defer os.RemoveAll(dir)
	ds, err := NewStore(dir)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	tests := []struct {
		acidefs []acidef
		tests   []test
	}{
		{
			[]acidef{
				{
					`{
						"acKind": "ImageManifest",
						"acVersion": "0.1.1",
						"name": "example.com/test01"
					}`,
					false,
				},
				{
					`{
						"acKind": "ImageManifest",
						"acVersion": "0.1.1",
						"name": "example.com/test02",
						"labels": [
							{
								"name": "version",
								"value": "1.0.0"
							}
						]
					}`,
					true,
				},
				{
					`{
						"acKind": "ImageManifest",
						"acVersion": "0.1.1",
						"name": "example.com/test02",
						"labels": [
							{
								"name": "version",
								"value": "2.0.0"
							}
						]
					}`,
					false,
				},
			},
			[]test{
				{
					"example.com/unexistentaci",
					types.Labels{},
					-1,
				},
				{
					"example.com/test01",
					types.Labels{},
					0,
				},
				{
					"example.com/test02",
					types.Labels{
						{
							Name:  "version",
							Value: "1.0.0",
						},
					},
					1,
				},
				{
					"example.com/test02",
					types.Labels{
						{
							Name:  "version",
							Value: "2.0.0",
						},
					},
					2,
				},
				{
					"example.com/test02",
					types.Labels{},
					1,
				},
			},
		},
	}

	for _, tt := range tests {
		keys := []string{}
		// Create ACIs
		for _, ad := range tt.acidefs {
			aci, err := aci.NewACI(dir, ad.imj, nil)
			if err != nil {
				t.Fatalf("error creating test tar: %v", err)
			}

			// Rewind the ACI
			if _, err := aci.Seek(0, 0); err != nil {
				t.Fatalf("unexpected error %v", err)
			}

			key, err := ds.WriteACI(aci, ad.latest)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			keys = append(keys, key)
		}

		for _, test := range tt.tests {
			key, err := ds.GetACI(test.name, test.labels)
			if test.expected == -1 {
				if err == nil {
					t.Fatalf("Expected no key for appName %s, got %s", test.name, key)
				}

			} else {
				if err != nil {
					t.Fatalf("unexpected error on GetACI for name %s, labels: %v: %v", test.name, test.labels, err)
				}
				if keys[test.expected] != key {
					t.Errorf("expected key: %s, got %s. GetACI with name: %s, labels: %v", key, keys[test.expected], test.name, test.labels)
				}
			}
		}
	}
}
