// Copyright 2016 The Linux Foundation
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

package image

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnpackLayerDuplicateEntries(t *testing.T) {
	tmp1, err := ioutil.TempDir("", "test-dup")
	if err != nil {
		t.Fatal(err)
	}
	tarfile := filepath.Join(tmp1, "test.tar")
	f, err := os.Create(tarfile)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	defer os.RemoveAll(tmp1)
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)

	tw.WriteHeader(&tar.Header{Name: "test", Size: 4, Mode: 0600})
	io.Copy(tw, bytes.NewReader([]byte("test")))
	tw.WriteHeader(&tar.Header{Name: "test", Size: 5, Mode: 0600})
	io.Copy(tw, bytes.NewReader([]byte("test1")))
	tw.Close()
	gw.Close()

	r, err := os.Open(tarfile)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	tmp2, err := ioutil.TempDir("", "test-dest-unpack")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp2)
	if err := unpackLayer(tmp2, r); err != nil && !strings.Contains(err.Error(), "duplicate entry for") {
		t.Fatalf("Expected to fail with duplicate entry, got %v", err)
	}
}

func TestUnpackLayer(t *testing.T) {
	tmp1, err := ioutil.TempDir("", "test-layer")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp1)
	err = os.MkdirAll(filepath.Join(tmp1, "blobs", "sha256"), 0700)
	if err != nil {
		t.Fatal(err)
	}
	tarfile := filepath.Join(tmp1, "blobs", "sha256", "test.tar")
	f, err := os.Create(tarfile)
	if err != nil {
		t.Fatal(err)
	}

	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)

	tw.WriteHeader(&tar.Header{Name: "test", Size: 4, Mode: 0600})
	io.Copy(tw, bytes.NewReader([]byte("test")))
	tw.Close()
	gw.Close()
	f.Close()

	// generate sha256 hash
	h := sha256.New()
	file, err := os.Open(tarfile)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	_, err = io.Copy(h, file)
	if err != nil {
		t.Fatal(err)
	}
	err = os.Rename(tarfile, filepath.Join(tmp1, "blobs", "sha256", fmt.Sprintf("%x", h.Sum(nil))))
	if err != nil {
		t.Fatal(err)
	}

	testManifest := manifest{
		Layers: []descriptor{descriptor{
			MediaType: "application/vnd.oci.image.layer.v1.tar+gzip",
			Digest:    fmt.Sprintf("sha256:%s", fmt.Sprintf("%x", h.Sum(nil))),
		}},
	}
	err = testManifest.unpack(newPathWalker(tmp1), filepath.Join(tmp1, "rootfs"))
	if err != nil {
		t.Fatal(err)
	}

	_, err = os.Stat(filepath.Join(tmp1, "rootfs", "test"))
	if err != nil {
		t.Fatal(err)
	}
}
