// Copyright 2015 CoreOS, Inc.
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

package acirenderer

// Use acirender_test package name and dot import acirenderer to avoid circular
// dependencies between cas and acirenderer tests.

import (
	"archive/tar"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/appc/spec/schema"
	"github.com/appc/spec/schema/types"
)

const tstprefix = "acirenderer-test"

type testTarEntry struct {
	header   *tar.Header
	contents string
}

func newTestTar(entries []*testTarEntry, dir string) (string, error) {
	t, err := ioutil.TempFile(dir, "tar")
	if err != nil {
		return "", err
	}
	defer t.Close()
	tw := tar.NewWriter(t)
	for _, entry := range entries {
		// Add default mode
		if entry.header.Mode == 0 {
			if entry.header.Typeflag == tar.TypeDir {
				entry.header.Mode = 0755
			} else {
				entry.header.Mode = 0644
			}
		}
		if err := tw.WriteHeader(entry.header); err != nil {
			return "", err
		}
		if _, err := io.WriteString(tw, entry.contents); err != nil {
			return "", err
		}
	}
	if err := tw.Close(); err != nil {
		return "", err
	}
	return t.Name(), nil
}

type fileInfo struct {
	path     string
	typeflag byte
	size     int64
	mode     os.FileMode
}

func newTestACI(entries []*testTarEntry, dir string, ds *TestStore) (string, error) {
	testTarPath, err := newTestTar(entries, dir)
	if err != nil {
		return "", err
	}

	hash, err := ds.WriteACI(testTarPath)
	if err != nil {
		return "", err
	}

	return hash, nil
}

func addDependencies(imj string, deps ...types.Dependency) (string, error) {
	var im schema.ImageManifest
	err := im.UnmarshalJSON([]byte(imj))
	if err != nil {
		return "", err
	}

	for _, dep := range deps {
		im.Dependencies = append(im.Dependencies, dep)
	}
	imjb, err := im.MarshalJSON()
	return string(imjb), err
}

// Test an image with 1 dep. The parent provides a dir not provided by the image.
func TestDirFromParent(t *testing.T) {
	dir, err := ioutil.TempDir("", tstprefix)
	if err != nil {
		t.Fatalf("error creating tempdir: %v", err)
	}
	defer os.RemoveAll(dir)
	ds := NewTestStore()

	imj := `
		{
		    "acKind": "ImageManifest",
		    "acVersion": "0.1.1",
		    "name": "example.com/test01"
		}
	`

	entries := []*testTarEntry{
		{
			contents: imj,
			header: &tar.Header{
				Name: "manifest",
				Size: int64(len(imj)),
			},
		},
		// An empty dir
		{
			header: &tar.Header{
				Name:     "rootfs/a",
				Typeflag: tar.TypeDir,
			},
		},
	}

	hash1, err := newTestACI(entries, dir, ds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	imj = `
		{
		    "acKind": "ImageManifest",
		    "acVersion": "0.1.1",
		    "name": "example.com/test02"
		}
	`

	h1, _ := types.NewHash(hash1)
	imj, err = addDependencies(imj,
		types.Dependency{
			App:     "example.com/test01",
			ImageID: h1},
	)

	entries = []*testTarEntry{
		{
			contents: imj,
			header: &tar.Header{
				Name: "manifest",
				Size: int64(len(imj)),
			},
		},
	}

	expectedFiles := []*fileInfo{
		&fileInfo{path: "manifest", typeflag: tar.TypeReg},
		&fileInfo{path: "rootfs", typeflag: tar.TypeDir},
		&fileInfo{path: "rootfs/a", typeflag: tar.TypeDir},
	}

	_, err = newTestACI(entries, dir, ds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = checkRenderACI("example.com/test02", expectedFiles, ds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// Test an image with 1 dep. The image provides a dir not provided by the parent.
func TestNewDir(t *testing.T) {
	dir, err := ioutil.TempDir("", tstprefix)
	if err != nil {
		t.Fatalf("error creating tempdir: %v", err)
	}
	defer os.RemoveAll(dir)
	ds := NewTestStore()

	imj := `
		{
		    "acKind": "ImageManifest",
		    "acVersion": "0.1.1",
		    "name": "example.com/test01"
		}
	`

	entries := []*testTarEntry{
		{
			contents: imj,
			header: &tar.Header{
				Name: "manifest",
				Size: int64(len(imj)),
			},
		},
		// An empty dir
		{
			header: &tar.Header{
				Name:     "rootfs/a",
				Typeflag: tar.TypeDir,
			},
		},
	}

	hash1, err := newTestACI(entries, dir, ds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	imj = `
		{
		    "acKind": "ImageManifest",
		    "acVersion": "0.1.1",
		    "name": "example.com/test02"
		}
	`

	h1, _ := types.NewHash(hash1)
	imj, err = addDependencies(imj,
		types.Dependency{
			App:     "example.com/test01",
			ImageID: h1},
	)

	entries = []*testTarEntry{
		{
			contents: imj,
			header: &tar.Header{
				Name: "manifest",
				Size: int64(len(imj)),
			},
		},
	}

	expectedFiles := []*fileInfo{
		&fileInfo{path: "manifest", typeflag: tar.TypeReg},
		&fileInfo{path: "rootfs", typeflag: tar.TypeDir},
		&fileInfo{path: "rootfs/a", typeflag: tar.TypeDir},
	}

	_, err = newTestACI(entries, dir, ds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = checkRenderACI("example.com/test02", expectedFiles, ds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// Test an image with 1 dep. The image overrides dirs modes from the parent dep. Verifies the right permissions.
func TestDirOverride(t *testing.T) {
	dir, err := ioutil.TempDir("", tstprefix)
	if err != nil {
		t.Fatalf("error creating tempdir: %v", err)
	}
	defer os.RemoveAll(dir)
	ds := NewTestStore()

	imj := `
		{
		    "acKind": "ImageManifest",
		    "acVersion": "0.1.1",
		    "name": "example.com/test01"
		}
	`

	entries := []*testTarEntry{
		{
			contents: imj,
			header: &tar.Header{
				Name: "manifest",
				Size: int64(len(imj)),
			},
		},
		{
			header: &tar.Header{
				Name:     "rootfs/a",
				Typeflag: tar.TypeDir,
			},
		},
	}

	hash1, err := newTestACI(entries, dir, ds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	imj = `
		{
		    "acKind": "ImageManifest",
		    "acVersion": "0.1.1",
		    "name": "example.com/test02"
		}
	`

	h1, _ := types.NewHash(hash1)
	imj, err = addDependencies(imj,
		types.Dependency{
			App:     "example.com/test01",
			ImageID: h1},
	)

	entries = []*testTarEntry{
		{
			contents: imj,
			header: &tar.Header{
				Name: "manifest",
				Size: int64(len(imj)),
			},
		},
		// An empty dir
		{
			header: &tar.Header{
				Name:     "rootfs/a",
				Typeflag: tar.TypeDir,
				Mode:     0700,
			},
		},
	}

	expectedFiles := []*fileInfo{
		&fileInfo{path: "manifest", typeflag: tar.TypeReg},
		&fileInfo{path: "rootfs", typeflag: tar.TypeDir},
		&fileInfo{path: "rootfs/a", typeflag: tar.TypeDir, mode: 0700},
	}

	_, err = newTestACI(entries, dir, ds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = checkRenderACI("example.com/test02", expectedFiles, ds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// Test an image with 1 dep. The parent provides a file not provided by the image.
func TestFileFromParent(t *testing.T) {
	dir, err := ioutil.TempDir("", tstprefix)
	if err != nil {
		t.Fatalf("error creating tempdir: %v", err)
	}
	defer os.RemoveAll(dir)
	ds := NewTestStore()

	imj := `
		{
		    "acKind": "ImageManifest",
		    "acVersion": "0.1.1",
		    "name": "example.com/test01"
		}
	`

	entries := []*testTarEntry{
		{
			contents: imj,
			header: &tar.Header{
				Name: "manifest",
				Size: int64(len(imj)),
			},
		},
		{
			contents: "hello",
			header: &tar.Header{
				Name: "rootfs/a/file01.txt",
				Size: 5,
			},
		},
	}

	hash1, err := newTestACI(entries, dir, ds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	imj = `
		{
		    "acKind": "ImageManifest",
		    "acVersion": "0.1.1",
		    "name": "example.com/test02"
		}
	`

	h1, _ := types.NewHash(hash1)
	imj, err = addDependencies(imj,
		types.Dependency{
			App:     "example.com/test01",
			ImageID: h1},
	)

	entries = []*testTarEntry{
		{
			contents: imj,
			header: &tar.Header{
				Name: "manifest",
				Size: int64(len(imj)),
			},
		},
	}

	expectedFiles := []*fileInfo{
		&fileInfo{path: "manifest", typeflag: tar.TypeReg},
		&fileInfo{path: "rootfs", typeflag: tar.TypeDir},
		&fileInfo{path: "rootfs/a", typeflag: tar.TypeDir},
		&fileInfo{path: "rootfs/a/file01.txt", typeflag: tar.TypeReg, size: 5},
	}

	_, err = newTestACI(entries, dir, ds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = checkRenderACI("example.com/test02", expectedFiles, ds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// Test an image with 1 dep. The image provides a file not provided by the parent.
func TestNewFile(t *testing.T) {
	dir, err := ioutil.TempDir("", tstprefix)
	if err != nil {
		t.Fatalf("error creating tempdir: %v", err)
	}
	defer os.RemoveAll(dir)
	ds := NewTestStore()

	imj := `
		{
		    "acKind": "ImageManifest",
		    "acVersion": "0.1.1",
		    "name": "example.com/test01"
		}
	`

	entries := []*testTarEntry{
		{
			contents: imj,
			header: &tar.Header{
				Name: "manifest",
				Size: int64(len(imj)),
			},
		},
	}

	hash1, err := newTestACI(entries, dir, ds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	imj = `
		{
		    "acKind": "ImageManifest",
		    "acVersion": "0.1.1",
		    "name": "example.com/test02"
		}
	`

	h1, _ := types.NewHash(hash1)
	imj, err = addDependencies(imj,
		types.Dependency{
			App:     "example.com/test01",
			ImageID: h1},
	)

	entries = []*testTarEntry{
		{
			contents: imj,
			header: &tar.Header{
				Name: "manifest",
				Size: int64(len(imj)),
			},
		},
		{
			contents: "hellohello",
			header: &tar.Header{
				Name: "rootfs/a/file01.txt",
				Size: 10,
			},
		},
	}

	expectedFiles := []*fileInfo{
		&fileInfo{path: "manifest", typeflag: tar.TypeReg},
		&fileInfo{path: "rootfs", typeflag: tar.TypeDir},
		&fileInfo{path: "rootfs/a", typeflag: tar.TypeDir},
		&fileInfo{path: "rootfs/a/file01.txt", typeflag: tar.TypeReg, size: 10},
	}

	_, err = newTestACI(entries, dir, ds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = checkRenderACI("example.com/test02", expectedFiles, ds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// Test an image with 1 dep. The image overrides a file already provided by the parent dep.
func TestFileOverride(t *testing.T) {
	dir, err := ioutil.TempDir("", tstprefix)
	if err != nil {
		t.Fatalf("error creating tempdir: %v", err)
	}
	defer os.RemoveAll(dir)
	ds := NewTestStore()

	imj := `
		{
		    "acKind": "ImageManifest",
		    "acVersion": "0.1.1",
		    "name": "example.com/test01"
		}
	`

	entries := []*testTarEntry{
		{
			contents: imj,
			header: &tar.Header{
				Name: "manifest",
				Size: int64(len(imj)),
			},
		},
		{
			contents: "hello",
			header: &tar.Header{
				Name: "rootfs/a/file01.txt",
				Size: 5,
			},
		},
	}

	hash1, err := newTestACI(entries, dir, ds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	imj = `
		{
		    "acKind": "ImageManifest",
		    "acVersion": "0.1.1",
		    "name": "example.com/test02"
		}
	`

	h1, _ := types.NewHash(hash1)
	imj, err = addDependencies(imj,
		types.Dependency{
			App:     "example.com/test01",
			ImageID: h1},
	)

	entries = []*testTarEntry{
		{
			contents: imj,
			header: &tar.Header{
				Name: "manifest",
				Size: int64(len(imj)),
			},
		},
		{
			contents: "hellohello",
			header: &tar.Header{
				Name: "rootfs/a/file01.txt",
				Size: 10,
			},
		},
	}

	expectedFiles := []*fileInfo{
		&fileInfo{path: "manifest", typeflag: tar.TypeReg},
		&fileInfo{path: "rootfs", typeflag: tar.TypeDir},
		&fileInfo{path: "rootfs/a", typeflag: tar.TypeDir},
		&fileInfo{path: "rootfs/a/file01.txt", typeflag: tar.TypeReg, size: 10},
	}

	_, err = newTestACI(entries, dir, ds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = checkRenderACI("example.com/test02", expectedFiles, ds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPWLOnlyParent(t *testing.T) {
	dir, err := ioutil.TempDir("", tstprefix)
	if err != nil {
		t.Fatalf("error creating tempdir: %v", err)
	}
	defer os.RemoveAll(dir)
	ds := NewTestStore()

	imj := `
		{
		    "acKind": "ImageManifest",
		    "acVersion": "0.1.1",
		    "name": "example.com/test01",
		    "pathWhitelist" : [ "/a/file01.txt", "/a/file02.txt", "/b/link01.txt", "/c/", "/d/" ]
		}
	`

	entries := []*testTarEntry{
		{
			contents: imj,
			header: &tar.Header{
				Name: "manifest",
				Size: int64(len(imj)),
			},
		},
		{
			contents: "hello",
			header: &tar.Header{
				Name: "rootfs/a/file01.txt",
				Size: 5,
			},
		},
		{
			contents: "hello",
			header: &tar.Header{
				Name: "rootfs/a/file02.txt",
				Size: 5,
			},
		},
		// It should not appear in rendered aci
		{
			contents: "hello",
			header: &tar.Header{
				Name: "rootfs/a/file03.txt",
				Size: 5,
			},
		},
		{
			header: &tar.Header{
				Name:     "rootfs/b/link01.txt",
				Linkname: "file01.txt",
				Typeflag: tar.TypeSymlink,
			},
		},
		// The files should not appear and a new file02.txt should appear but the directory should be left with its permissions
		{
			header: &tar.Header{
				Name:     "rootfs/c",
				Typeflag: tar.TypeDir,
				Mode:     0700,
			},
		},
		{
			contents: "hello",
			header: &tar.Header{
				Name: "rootfs/c/file01.txt",
				Size: 5,
				Mode: 0700,
			},
		},
		// The files should not appear but the directory should be left and also its permissions
		{
			header: &tar.Header{
				Name:     "rootfs/d",
				Typeflag: tar.TypeDir,
				Mode:     0700,
			},
		},
		{
			contents: "hello",
			header: &tar.Header{
				Name: "rootfs/d/file01.txt",
				Size: 5,
				Mode: 0700,
			},
		},
		// The files and the directory should not appear
		{
			contents: "hello",
			header: &tar.Header{
				Name: "rootfs/e/file01.txt",
				Size: 5,
				Mode: 0700,
			},
		},
	}

	hash1, err := newTestACI(entries, dir, ds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	imj = `
		{
		    "acKind": "ImageManifest",
		    "acVersion": "0.1.1",
		    "name": "example.com/test02"
		}
	`

	h1, _ := types.NewHash(hash1)
	imj, err = addDependencies(imj,
		types.Dependency{
			App:     "example.com/test01",
			ImageID: h1},
	)

	entries = []*testTarEntry{
		{
			contents: imj,
			header: &tar.Header{
				Name: "manifest",
				Size: int64(len(imj)),
			},
		},
		{
			contents: "hellohello",
			header: &tar.Header{
				Name: "rootfs/b/file01.txt",
				Size: 10,
			},
		},
		// New file
		{
			contents: "hello",
			header: &tar.Header{
				Name: "rootfs/c/file02.txt",
				Size: 5,
			},
		},
	}

	expectedFiles := []*fileInfo{
		&fileInfo{path: "manifest", typeflag: tar.TypeReg},
		&fileInfo{path: "rootfs", typeflag: tar.TypeDir},
		&fileInfo{path: "rootfs/a", typeflag: tar.TypeDir},
		&fileInfo{path: "rootfs/a/file01.txt", typeflag: tar.TypeReg, size: 5},
		&fileInfo{path: "rootfs/a/file02.txt", typeflag: tar.TypeReg, size: 5},
		&fileInfo{path: "rootfs/b", typeflag: tar.TypeDir},
		&fileInfo{path: "rootfs/b/link01.txt", typeflag: tar.TypeSymlink},
		&fileInfo{path: "rootfs/b/file01.txt", typeflag: tar.TypeReg, size: 10},
		&fileInfo{path: "rootfs/c", typeflag: tar.TypeDir, mode: 0700},
		&fileInfo{path: "rootfs/c/file02.txt", typeflag: tar.TypeReg, size: 5},
		&fileInfo{path: "rootfs/d", typeflag: tar.TypeDir, mode: 0700},
	}

	_, err = newTestACI(entries, dir, ds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = checkRenderACI("example.com/test02", expectedFiles, ds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPWLOnlyImage(t *testing.T) {
	dir, err := ioutil.TempDir("", tstprefix)
	if err != nil {
		t.Fatalf("error creating tempdir: %v", err)
	}
	defer os.RemoveAll(dir)
	ds := NewTestStore()

	imj := `
		{
		    "acKind": "ImageManifest",
		    "acVersion": "0.1.1",
		    "name": "example.com/test01"
		}
	`

	entries := []*testTarEntry{
		{
			contents: imj,
			header: &tar.Header{
				Name: "manifest",
				Size: int64(len(imj)),
			},
		},
		// It should be overriden by the one provided by the upper image in rendered aci
		{
			contents: "hello",
			header: &tar.Header{
				Name: "rootfs/a/file01.txt",
				Size: 5,
			},
		},
		{
			contents: "hello",
			header: &tar.Header{
				Name: "rootfs/a/file02.txt",
				Size: 5,
			},
		},
		// It should not appear in rendered aci
		{
			contents: "hello",
			header: &tar.Header{
				Name: "rootfs/a/file03.txt",
				Size: 5,
			},
		},
		{
			header: &tar.Header{
				Name:     "rootfs/b/link01.txt",
				Linkname: "file01.txt",
				Typeflag: tar.TypeSymlink,
			},
		},
		// The files should not appear and a new file02.txt should appear but the directory should be left with its permissions
		{
			header: &tar.Header{
				Name:     "rootfs/c",
				Typeflag: tar.TypeDir,
				Mode:     0700,
			},
		},
		{
			contents: "hello",
			header: &tar.Header{
				Name: "rootfs/c/file01.txt",
				Size: 5,
				Mode: 0700,
			},
		},
		// The files should not appear but the directory should be left and also its permissions
		{
			header: &tar.Header{
				Name:     "rootfs/d",
				Typeflag: tar.TypeDir,
				Mode:     0700,
			},
		},
		{
			contents: "hello",
			header: &tar.Header{
				Name: "rootfs/d/file01.txt",
				Size: 5,
				Mode: 0700,
			},
		},
		// The files and the directory should not appear
		{
			contents: "hello",
			header: &tar.Header{
				Name: "rootfs/e/file01.txt",
				Size: 5,
				Mode: 0700,
			},
		},
	}

	hash1, err := newTestACI(entries, dir, ds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	imj = `
		{
		    "acKind": "ImageManifest",
		    "acVersion": "0.1.1",
		    "name": "example.com/test02",
		    "pathWhitelist" : [ "/a/file01.txt", "/a/file02.txt", "/b/link01.txt", "/b/file01.txt", "/c/file02.txt", "/d/" ]
		}
	`

	h1, _ := types.NewHash(hash1)
	imj, err = addDependencies(imj,
		types.Dependency{
			App:     "example.com/test01",
			ImageID: h1},
	)

	entries = []*testTarEntry{
		{
			contents: imj,
			header: &tar.Header{
				Name: "manifest",
				Size: int64(len(imj)),
			},
		},
		{
			contents: "hellohello",
			header: &tar.Header{
				Name: "rootfs/b/file01.txt",
				Size: 10,
			},
		},
		// New file
		{
			contents: "hello",
			header: &tar.Header{
				Name: "rootfs/c/file02.txt",
				Size: 5,
			},
		},
	}

	expectedFiles := []*fileInfo{
		&fileInfo{path: "manifest", typeflag: tar.TypeReg},
		&fileInfo{path: "rootfs", typeflag: tar.TypeDir},
		&fileInfo{path: "rootfs/a", typeflag: tar.TypeDir},
		&fileInfo{path: "rootfs/a/file01.txt", typeflag: tar.TypeReg, size: 5},
		&fileInfo{path: "rootfs/a/file02.txt", typeflag: tar.TypeReg, size: 5},
		&fileInfo{path: "rootfs/b", typeflag: tar.TypeDir},
		&fileInfo{path: "rootfs/b/link01.txt", typeflag: tar.TypeSymlink},
		&fileInfo{path: "rootfs/b/file01.txt", typeflag: tar.TypeReg, size: 10},
		&fileInfo{path: "rootfs/c", typeflag: tar.TypeDir, mode: 0700},
		&fileInfo{path: "rootfs/c/file02.txt", typeflag: tar.TypeReg, size: 5},
		&fileInfo{path: "rootfs/d", typeflag: tar.TypeDir, mode: 0700},
	}

	_, err = newTestACI(entries, dir, ds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = checkRenderACI("example.com/test02", expectedFiles, ds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// Test an image with a pathwhitelist and 2 deps (first without pathWhiteList and the second with pathWhiteList)
func Test2Deps1(t *testing.T) {
	dir, err := ioutil.TempDir("", tstprefix)
	if err != nil {
		t.Fatalf("error creating tempdir: %v", err)
	}
	defer os.RemoveAll(dir)
	ds := NewTestStore()

	imj := `
		{
		    "acKind": "ImageManifest",
		    "acVersion": "0.1.1",
		    "name": "example.com/test01",
		    "pathWhitelist" : [ "/a/file01.txt", "/a/file02.txt", "/a/file03.txt", "/a/file04.txt", "/b/link01.txt", "/b/file01.txt" ]
		}
	`

	entries := []*testTarEntry{
		{
			contents: imj,
			header: &tar.Header{
				Name: "manifest",
				Size: int64(len(imj)),
			},
		},
		// It should be overriden by the one provided by the upper image
		{
			contents: "hello",
			header: &tar.Header{
				Name: "rootfs/a/file01.txt",
				Size: 5,
			},
		},
		// It should be overriden by the one provided by the next dep
		{
			contents: "hello",
			header: &tar.Header{
				Name: "rootfs/a/file02.txt",
				Size: 5,
			},
		},
		// It should remain this
		{
			contents: "hello",
			header: &tar.Header{
				Name: "rootfs/a/file03.txt",
				Size: 5,
			},
		},
		// It should not appear in rendered aci
		{
			contents: "hello",
			header: &tar.Header{
				Name: "rootfs/a/file04.txt",
				Size: 5,
			},
		},
		{
			header: &tar.Header{
				Name:     "rootfs/b/link01.txt",
				Linkname: "file01.txt",
				Typeflag: tar.TypeSymlink,
			},
		},
	}

	hash1, err := newTestACI(entries, dir, ds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	imj = `
		{
		    "acKind": "ImageManifest",
		    "acVersion": "0.1.1",
		    "name": "example.com/test02"
		}
	`

	entries = []*testTarEntry{
		{
			contents: imj,
			header: &tar.Header{
				Name: "manifest",
				Size: int64(len(imj)),
			},
		},
		// It should be overriden by the one provided by the upper image
		{
			contents: "hellohello",
			header: &tar.Header{
				Name: "rootfs/a/file01.txt",
				Size: 10,
			},
		},
		{
			contents: "hellohello",
			header: &tar.Header{
				Name: "rootfs/a/file02.txt",
				Size: 10,
			},
		},
		{
			contents: "hello",
			header: &tar.Header{
				Name: "rootfs/b/file01.txt",
				Size: 5,
			},
		},
	}

	hash2, err := newTestACI(entries, dir, ds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	imj = `
		{
		    "acKind": "ImageManifest",
		    "acVersion": "0.1.1",
		    "name": "example.com/test03",
		    "pathWhitelist" : [ "/a/file01.txt", "/a/file02.txt", "/a/file03.txt", "/b/link01.txt", "/b/file01.txt", "/b/file02.txt", "/c/file01.txt" ]
		}
	`

	h1, _ := types.NewHash(hash1)
	h2, _ := types.NewHash(hash2)
	imj, err = addDependencies(imj,
		types.Dependency{
			App:     "example.com/test01",
			ImageID: h1},
		types.Dependency{
			App:     "example.com/test02",
			ImageID: h2},
	)

	entries = []*testTarEntry{
		{
			contents: imj,
			header: &tar.Header{
				Name: "manifest",
				Size: int64(len(imj)),
			},
		},
		// Overridden
		{
			contents: "hellohellohello",
			header: &tar.Header{
				Name: "rootfs/a/file01.txt",
				Size: 15,
			},
		},
		{
			contents: "hello",
			header: &tar.Header{
				Name: "rootfs/b/file02.txt",
				Size: 5,
			},
		},
		{
			contents: "hello",
			header: &tar.Header{
				Name: "rootfs/c/file01.txt",
				Size: 5,
			},
		},
	}

	_, err = newTestACI(entries, dir, ds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedFiles := []*fileInfo{
		&fileInfo{path: "manifest", typeflag: tar.TypeReg},
		&fileInfo{path: "rootfs", typeflag: tar.TypeDir},
		&fileInfo{path: "rootfs/a", typeflag: tar.TypeDir},
		&fileInfo{path: "rootfs/a/file01.txt", typeflag: tar.TypeReg, size: 15},
		&fileInfo{path: "rootfs/a/file02.txt", typeflag: tar.TypeReg, size: 10},
		&fileInfo{path: "rootfs/a/file03.txt", typeflag: tar.TypeReg, size: 5},
		&fileInfo{path: "rootfs/b", typeflag: tar.TypeDir},
		&fileInfo{path: "rootfs/b/link01.txt", typeflag: tar.TypeSymlink},
		&fileInfo{path: "rootfs/b/file01.txt", typeflag: tar.TypeReg, size: 5},
		&fileInfo{path: "rootfs/b/file02.txt", typeflag: tar.TypeReg, size: 5},
		&fileInfo{path: "rootfs/c", typeflag: tar.TypeDir},
		&fileInfo{path: "rootfs/c/file01.txt", typeflag: tar.TypeReg, size: 5},
	}

	err = checkRenderACI("example.com/test03", expectedFiles, ds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// Test an image with a pathwhitelist and 2 deps (first without pathWhiteList and the second with pathWhiteList)
func Test2Deps2(t *testing.T) {
	dir, err := ioutil.TempDir("", tstprefix)
	if err != nil {
		t.Fatalf("error creating tempdir: %v", err)
	}
	defer os.RemoveAll(dir)
	ds := NewTestStore()

	imj := `
		{
		    "acKind": "ImageManifest",
		    "acVersion": "0.1.1",
		    "name": "example.com/test01"
		}
	`

	entries := []*testTarEntry{
		{
			contents: imj,
			header: &tar.Header{
				Name: "manifest",
				Size: int64(len(imj)),
			},
		},
		// It should be overriden by the one provided by the upper image
		{
			contents: "hello",
			header: &tar.Header{
				Name: "rootfs/a/file01.txt",
				Size: 5,
			},
		},
		// It should be overriden by the one provided by the next dep
		{
			contents: "hello",
			header: &tar.Header{
				Name: "rootfs/a/file02.txt",
				Size: 5,
			},
		},
		// It should remain this
		{
			contents: "hello",
			header: &tar.Header{
				Name: "rootfs/a/file03.txt",
				Size: 5,
			},
		},
		// It should not appear in rendered aci
		{
			contents: "hello",
			header: &tar.Header{
				Name: "rootfs/a/file04.txt",
				Size: 5,
			},
		},
		{
			header: &tar.Header{
				Name:     "rootfs/b/link01.txt",
				Linkname: "file01.txt",
				Typeflag: tar.TypeSymlink,
			},
		},
	}

	hash1, err := newTestACI(entries, dir, ds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	imj = `
		{
		    "acKind": "ImageManifest",
		    "acVersion": "0.1.1",
		    "name": "example.com/test02",
		    "pathWhitelist" : [ "/a/file01.txt", "/a/file02.txt", "/b/file01.txt" ]
		}
	`

	entries = []*testTarEntry{
		{
			contents: imj,
			header: &tar.Header{
				Name: "manifest",
				Size: int64(len(imj)),
			},
		},
		// It should be overriden by the one provided by the upper image
		{
			contents: "hellohello",
			header: &tar.Header{
				Name: "rootfs/a/file01.txt",
				Size: 10,
			},
		},
		{
			contents: "hellohello",
			header: &tar.Header{
				Name: "rootfs/a/file02.txt",
				Size: 10,
			},
		},
		{
			contents: "hello",
			header: &tar.Header{
				Name: "rootfs/b/file01.txt",
				Size: 5,
			},
		},
	}

	hash2, err := newTestACI(entries, dir, ds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	imj = `
		{
		    "acKind": "ImageManifest",
		    "acVersion": "0.1.1",
		    "name": "example.com/test03",
		    "pathWhitelist" : [ "/a/file01.txt", "/a/file02.txt", "/a/file03.txt", "/b/link01.txt", "/b/file01.txt", "/b/file02.txt", "/c/file01.txt" ]
		}
	`

	h1, _ := types.NewHash(hash1)
	h2, _ := types.NewHash(hash2)
	imj, err = addDependencies(imj,
		types.Dependency{
			App:     "example.com/test01",
			ImageID: h1},
		types.Dependency{
			App:     "example.com/test02",
			ImageID: h2},
	)

	entries = []*testTarEntry{
		{
			contents: imj,
			header: &tar.Header{
				Name: "manifest",
				Size: int64(len(imj)),
			},
		},
		// Overridden
		{
			contents: "hellohellohello",
			header: &tar.Header{
				Name: "rootfs/a/file01.txt",
				Size: 15,
			},
		},
		{
			contents: "hello",
			header: &tar.Header{
				Name: "rootfs/b/file02.txt",
				Size: 5,
			},
		},
		{
			contents: "hello",
			header: &tar.Header{
				Name: "rootfs/c/file01.txt",
				Size: 5,
			},
		},
	}

	_, err = newTestACI(entries, dir, ds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedFiles := []*fileInfo{
		&fileInfo{path: "manifest", typeflag: tar.TypeReg},
		&fileInfo{path: "rootfs", typeflag: tar.TypeDir},
		&fileInfo{path: "rootfs/a", typeflag: tar.TypeDir},
		&fileInfo{path: "rootfs/a/file01.txt", typeflag: tar.TypeReg, size: 15},
		&fileInfo{path: "rootfs/a/file02.txt", typeflag: tar.TypeReg, size: 10},
		&fileInfo{path: "rootfs/a/file03.txt", typeflag: tar.TypeReg, size: 5},
		&fileInfo{path: "rootfs/b", typeflag: tar.TypeDir},
		&fileInfo{path: "rootfs/b/link01.txt", typeflag: tar.TypeSymlink},
		&fileInfo{path: "rootfs/b/file01.txt", typeflag: tar.TypeReg, size: 5},
		&fileInfo{path: "rootfs/b/file02.txt", typeflag: tar.TypeReg, size: 5},
		&fileInfo{path: "rootfs/c", typeflag: tar.TypeDir},
		&fileInfo{path: "rootfs/c/file01.txt", typeflag: tar.TypeReg, size: 5},
	}

	err = checkRenderACI("example.com/test03", expectedFiles, ds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func checkRenderACI(app types.ACName, expectedFiles []*fileInfo, ds *TestStore) error {

	imagedir, err := ioutil.TempDir("", "imagedir")
	if err != nil {
		return err
	}
	defer os.RemoveAll(imagedir)
	err = RenderACI(app, nil, imagedir, ds)
	if err != nil {
		return err
	}
	err = checkExpectedFiles(imagedir, FISliceToMap(expectedFiles))
	if err != nil {
		return err
	}

	return nil

}
func checkExpectedFiles(dir string, expectedFiles map[string]*fileInfo) error {

	files := make(map[string]*fileInfo)
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		fm := info.Mode()
		if path == dir {
			return nil
		}
		relpath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		switch {
		case fm.IsRegular():
			files[relpath] = &fileInfo{path: relpath, typeflag: tar.TypeReg, size: info.Size(), mode: info.Mode().Perm()}
		case info.IsDir():
			files[relpath] = &fileInfo{path: relpath, typeflag: tar.TypeDir, mode: info.Mode().Perm()}
		case fm&os.ModeSymlink != 0:
			files[relpath] = &fileInfo{path: relpath, typeflag: tar.TypeSymlink, mode: info.Mode()}
		default:
			return fmt.Errorf("not handled file mode %v", fm)
		}

		return nil
	})
	if err != nil {
		return err
	}

	// Set defaults for not specified expected file mode
	for _, ef := range expectedFiles {
		if ef.mode == 0 {
			if ef.typeflag == tar.TypeDir {
				ef.mode = 0755
			} else {
				ef.mode = 0644
			}
		}
	}

	for _, ef := range expectedFiles {
		_, ok := files[ef.path]
		if !ok {
			return fmt.Errorf("Expected file \"%s\" not in files", ef.path)
		}

	}

	for _, file := range files {
		ef, ok := expectedFiles[file.path]
		if !ok {
			return fmt.Errorf("file \"%s\" not in expectedFiles", file.path)
		}
		if ef.typeflag != file.typeflag {
			return fmt.Errorf("file \"%s\": file type differs: found %d, wanted: %d", file.path, file.typeflag, ef.typeflag)
		}
		if ef.typeflag == tar.TypeReg && file.path != "manifest" {
			if ef.size != file.size {
				return fmt.Errorf("file \"%s\": size differs: found %d, wanted: %d", file.path, file.size, ef.size)
			}
		}
		// Check modes but ignore symlinks
		if ef.mode != file.mode && ef.typeflag != tar.TypeSymlink {
			return fmt.Errorf("file \"%s\": mode differs: found %#o, wanted: %#o", file.path, file.mode, ef.mode)
		}

	}
	return nil
}

func FISliceToMap(slice []*fileInfo) map[string]*fileInfo {
	fim := make(map[string]*fileInfo, len(slice))
	for _, fi := range slice {
		fim[fi.path] = fi
	}
	return fim
}
