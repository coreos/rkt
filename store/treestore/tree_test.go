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

package treestore

import (
	"archive/tar"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	aciimage "github.com/coreos/rkt/common/image/aci"
	"github.com/coreos/rkt/pkg/aci"
	"github.com/coreos/rkt/pkg/sys"
	"github.com/coreos/rkt/store/casref/rwcasref"
	"github.com/coreos/rkt/store/manifestcache"
)

const tstprefix = "treestore-test"

// TODO(sgotti) when the TreeStore will use an interface, change it to a
// test implementation without relying on store/imagestore
func testStoreWriteACI(dir string, s *rwcasref.Store) (string, error) {
	imj := `
		{
		    "acKind": "ImageManifest",
		    "acVersion": "0.8.6",
		    "name": "example.com/test01"
		}
	`

	entries := []*aci.ACIEntry{
		// An empty dir
		{
			Header: &tar.Header{
				Name:     "rootfs/a",
				Typeflag: tar.TypeDir,
			},
		},
		{
			Contents: "hello",
			Header: &tar.Header{
				Name: "hello.txt",
				Size: 5,
			},
		},
		{
			Header: &tar.Header{
				Name:     "rootfs/link.txt",
				Linkname: "rootfs/hello.txt",
				Typeflag: tar.TypeSymlink,
			},
		},
		// dangling symlink
		{
			Header: &tar.Header{
				Name:     "rootfs/link2.txt",
				Linkname: "rootfs/missingfile.txt",
				Typeflag: tar.TypeSymlink,
			},
		},
		{
			Header: &tar.Header{
				Name:     "rootfs/fifo",
				Typeflag: tar.TypeFifo,
			},
		},
	}
	aci, err := aci.NewACI(dir, imj, entries)
	if err != nil {
		return "", err
	}
	defer aci.Close()

	// Rewind the ACI
	if _, err := aci.Seek(0, 0); err != nil {
		return "", err
	}

	// Import the new ACI
	digest, err := aciimage.WriteACI(s, aci)
	if err != nil {
		return "", err
	}
	return digest, nil
}

func TestTreeStoreRender(t *testing.T) {
	if !sys.HasChrootCapability() {
		t.Skipf("chroot capability not available. Disabling test.")
	}

	dir, err := ioutil.TempDir("", tstprefix)
	if err != nil {
		t.Fatalf("error creating tempdir: %v", err)
	}
	defer os.RemoveAll(dir)

	s, err := rwcasref.NewStore(filepath.Join(dir, "casref"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mc, err := manifestcache.NewACIManifestCache(filepath.Join(dir, "manifestcache"), s)
	if err != nil {
		t.Fatalf("cannot open manifestcache: %v", err)
	}

	ts, err := NewStore(dir, s, mc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	digest, err := testStoreWriteACI(dir, s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	id := "treestoreid01"

	if err := ts.render(id, digest); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify image Hash. Should be the same.
	_, err = ts.Check(id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTreeStoreRemove(t *testing.T) {
	if !sys.HasChrootCapability() {
		t.Skipf("chroot capability not available. Disabling test.")
	}

	dir, err := ioutil.TempDir("", tstprefix)
	if err != nil {
		t.Fatalf("error creating tempdir: %v", err)
	}
	defer os.RemoveAll(dir)

	s, err := rwcasref.NewStore(dir)
	if err != nil {

		t.Fatalf("unexpected error: %v", err)
	}

	mc, err := manifestcache.NewACIManifestCache(filepath.Join(dir, "manifestcache"), s)
	if err != nil {
		t.Fatalf("cannot open manifestcache: %v", err)
	}

	ts, err := NewStore(dir, s, mc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	digest, err := testStoreWriteACI(dir, s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	id := "treestoreid01"

	// Test non existent dir
	err = ts.remove(id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Test rendered tree
	if err = ts.render(id, digest); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = ts.remove(id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTreeStore(t *testing.T) {
	if !sys.HasChrootCapability() {
		t.Skipf("chroot capability not available. Disabling test.")
	}

	dir, err := ioutil.TempDir("", tstprefix)
	if err != nil {
		t.Fatalf("error creating tempdir: %v", err)
	}
	defer os.RemoveAll(dir)

	s, err := rwcasref.NewStore(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mc, err := manifestcache.NewACIManifestCache(filepath.Join(dir, "manifestcache"), s)
	if err != nil {
		t.Fatalf("cannot open manifestcache: %v", err)
	}

	ts, err := NewStore(dir, s, mc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	imj := `
		{
		    "acKind": "ImageManifest",
		    "acVersion": "0.8.6",
		    "name": "example.com/test01"
		}
	`

	entries := []*aci.ACIEntry{
		// An empty dir
		{
			Header: &tar.Header{
				Name:     "rootfs/a",
				Typeflag: tar.TypeDir,
			},
		},
		{
			Contents: "hello",
			Header: &tar.Header{
				Name: "hello.txt",
				Size: 5,
			},
		},
		{
			Header: &tar.Header{
				Name:     "rootfs/link.txt",
				Linkname: "rootfs/hello.txt",
				Typeflag: tar.TypeSymlink,
			},
		},
		// dangling symlink
		{
			Header: &tar.Header{
				Name:     "rootfs/link2.txt",
				Linkname: "rootfs/missingfile.txt",
				Typeflag: tar.TypeSymlink,
			},
		},
		{
			Header: &tar.Header{
				Name:     "rootfs/fifo",
				Typeflag: tar.TypeFifo,
			},
		},
	}
	aci, err := aci.NewACI(dir, imj, entries)
	if err != nil {
		t.Fatalf("error creating test tar: %v", err)
	}
	defer aci.Close()

	// Rewind the ACI
	if _, err := aci.Seek(0, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Import the new ACI
	digest, err := aciimage.WriteACI(s, aci)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Ask the store to render the treestore
	id, err := ts.Render(digest, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify image Hash. Should be the same.
	_, err = ts.Check(id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Change a file permission
	rootfs := ts.GetRootFS(id)
	err = os.Chmod(filepath.Join(rootfs, "a"), 0600)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify image Hash. Should be different
	_, err = ts.Check(id)
	if err == nil {
		t.Errorf("expected non-nil error!")
	}

	// rebuild the tree
	prevID := id
	id, err = ts.Render(digest, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if id != prevID {
		t.Fatalf("unexpected different IDs. prevID: %s, id: %s", prevID, id)
	}

	// Add a file
	rootfs = ts.GetRootFS(id)
	err = ioutil.WriteFile(filepath.Join(rootfs, "newfile"), []byte("newfile"), 0644)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify image Hash. Should be different
	_, err = ts.Check(id)
	if err == nil {
		t.Errorf("expected non-nil error!")
	}
}
