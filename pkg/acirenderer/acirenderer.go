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

import (
	"archive/tar"
	"container/list"
	"crypto/sha512"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/appc/spec/schema"
	"github.com/appc/spec/schema/types"

	ptar "github.com/coreos/rocket/pkg/tar"
)

// An ACIProvider provides functions to search for an aci and get its contents
// This interface is temporary as the function names may change but its needed
// to avoid a dependency on cas.Store
type ACIProvider interface {
	GetImageManifest(key string) (*schema.ImageManifest, error)
	GetACI(name types.ACName, labels types.Labels) (string, error)
	ReadStream(key string) (io.ReadCloser, error)
	ResolveKey(key string) (string, error)
	HashToKey(h hash.Hash) string
}

// And Image contains the ImageManifest, the Hash and the Level in the dependency tree of this image
type Image struct {
	im    *schema.ImageManifest
	key   string
	level uint16
}

// An ordered slice made of Image structs. Represents a flatten dependency tree.
// The upper Image should be the first in the slice with a level of 0.
// For example if A is the upper images and has two deps (in order B and C). And C has one dep (D),
// the slice (reporting the app name and excluding im and Hash) should be:
// [{A, Level: 0}, {C, Level:1}, {D, Level: 2}, {B, Level: 1}]
type Images []Image

func CreateDepListFromImageID(key string, ap ACIProvider) (Images, error) {
	return createDepList(key, ap)
}

func CreateDepListFromNameLabels(name types.ACName, labels types.Labels, ap ACIProvider) (Images, error) {
	key, err := ap.GetACI(name, labels)
	if err != nil {
		return nil, err
	}
	return createDepList(key, ap)
}

// Returns an ordered list of Image type to be rendered
func createDepList(key string, ap ACIProvider) (Images, error) {
	imgsl := list.New()
	im, err := ap.GetImageManifest(key)
	if err != nil {
		return nil, err
	}

	img := Image{im: im, key: key, level: 0}
	imgsl.PushFront(img)

	// Create a flatten dependency tree. Use a LinkedList to be able to
	// insert elements in the list while working on it.
	for el := imgsl.Front(); el != nil; el = el.Next() {
		img := el.Value.(Image)
		dependencies := img.im.Dependencies
		for _, d := range dependencies {
			var depimg Image
			var depKey string
			if d.ImageID != nil && !d.ImageID.Empty() {
				depKey, err = ap.ResolveKey(d.ImageID.String())
				if err != nil {
					return nil, err
				}
			} else {
				var err error
				depKey, err = ap.GetACI(d.App, d.Labels)
				if err != nil {
					return nil, err
				}
			}
			im, err := ap.GetImageManifest(depKey)
			if err != nil {
				return nil, err
			}
			depimg = Image{im: im, key: depKey, level: img.level + 1}
			imgsl.InsertAfter(depimg, el)
		}
	}

	imgs := Images{}
	for el := imgsl.Front(); el != nil; el = el.Next() {
		imgs = append(imgs, el.Value.(Image))
	}
	return imgs, nil
}

func RenderACIWithImageID(imageID types.Hash, dir string, ap ACIProvider) error {
	key, err := ap.ResolveKey(imageID.String())
	if err != nil {
		return err
	}
	imgs, err := CreateDepListFromImageID(key, ap)
	if err != nil {
		return err
	}
	return renderACI(imgs, dir, ap)
}

// Given an image app name, optional labels and optional imageID get the best
// matching image available in the store, build its dependency list and
// render it inside dir
func RenderACI(name types.ACName, labels types.Labels, dir string, ap ACIProvider) error {
	imgs, err := CreateDepListFromNameLabels(name, labels, ap)
	if err != nil {
		return err
	}
	return renderACI(imgs, dir, ap)
}

func renderACI(imgs Images, dir string, ap ACIProvider) error {
	if len(imgs) == 0 {
		return fmt.Errorf("image list empty")
	}

	// This implementation needs to start from the end of the tree.
	end := len(imgs) - 1
	prevlevel := imgs[end].level
	for i := end; i >= 0; i-- {
		img := imgs[i]

		err := renderImage(img, dir, ap, prevlevel)
		if err != nil {
			return err
		}
		if img.level < prevlevel {
			prevlevel = img.level
		}
	}
	return nil
}

func renderImage(img Image, dir string, ap ACIProvider, prevlevel uint16) error {
	rs, err := ap.ReadStream(img.key)
	if err != nil {
		return err
	}
	defer rs.Close()

	hash := sha512.New()
	r := io.TeeReader(rs, hash)

	if err := ptar.ExtractTar(tar.NewReader(r), dir, true, pwlToMap(img.im.PathWhitelist)); err != nil {
		return fmt.Errorf("error extracting ACI: %v", err)
	}

	// Tar does not necessarily read the complete file, so ensure we read the entirety into the hash
	if _, err := io.Copy(ioutil.Discard, r); err != nil {
		return fmt.Errorf("error reading ACI: %v", err)
	}

	if g := ap.HashToKey(hash); g != img.key {
		if err := os.RemoveAll(dir); err != nil {
			fmt.Fprintf(os.Stderr, "error cleaning up directory: %v\n", err)
		}
		return fmt.Errorf("image hash does not match expected (%s != %s)", g, img.key)
	}
	// If the image is an a previous level remove files not in
	// PathWhitelist (if PathWhitelist isn't empty)
	// Directories are handled after file removal and all empty directories
	// not in the pathWhiteList will be removed
	if img.level < prevlevel {
		// Apply pathWhitelist only if it's not empty
		if len(img.im.PathWhitelist) != 0 {
			pwlm := pwlToMap(img.im.PathWhitelist)
			rootfs := filepath.Join(dir, "rootfs/")
			err = filepath.Walk(rootfs, func(path string, info os.FileInfo, err error) error {
				if info.IsDir() {
					return nil
				}

				relpath, err := filepath.Rel(dir, path)
				if err != nil {
					return err
				}
				if _, ok := pwlm[relpath]; !ok {
					err := os.Remove(path)
					if err != nil {
						return err
					}
				}
				return nil
			})
			if err != nil {
				return fmt.Errorf("error walking rootfs: %v", err)
			}

			removeEmptyDirs(dir, rootfs, pwlm)
		}
	}
	return nil
}

func removeEmptyDirs(base string, dir string, pathWhitelistMap map[string]struct{}) error {
	dirs, err := getDirectories(dir)
	if err != nil {
		return err
	}
	for _, dir := range dirs {
		removeEmptyDirs(base, dir, pathWhitelistMap)
	}
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	names, err := f.Readdirnames(-1)
	f.Close()
	if err != nil {
		return err
	}
	if len(names) == 0 {
		relpath, err := filepath.Rel(base, dir)
		if err != nil {
			return err
		}
		if _, ok := pathWhitelistMap[relpath]; !ok {
			err := os.Remove(dir)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func getDirectories(dir string) ([]string, error) {
	f, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	infos, err := f.Readdir(-1)
	f.Close()
	if err != nil {
		return nil, err
	}

	dirs := []string{}
	for _, info := range infos {
		if info.IsDir() {
			dirs = append(dirs, filepath.Join(dir, info.Name()))
		}
	}
	return dirs, nil
}

// Convert pathWhiteList slice to a map for faster search
// Also change path to be relative to "/" so it can easyly used without the
// calling function calling filepath.Join("/", ...)
// if pwl length is 0 return a nil map so ExtractTar won't apply any
// pathWhiteList filtering
func pwlToMap(pwl []string) map[string]struct{} {
	if len(pwl) == 0 {
		return nil
	}
	m := make(ptar.PathWhitelistMap, len(pwl))
	for _, v := range pwl {
		relpath := filepath.Join("rootfs/", v)
		m[relpath] = struct{}{}
	}
	return m
}
