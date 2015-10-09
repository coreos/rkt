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

package oci2aci

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/aci"
	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/schema"
)

func buildACI(dir string) (string, error) {
	imageName, err := filepath.Abs(dir)
	if err != nil {
		log.Fatalf("err: %v", err)
	}
	imageName += ".aci"
	err = createACI(dir, imageName)

	return imageName, err
}

func createACI(dir string, imageName string) error {
	var errStr string
	var errRes error
	buildNocompress := true
	root := dir
	tgt := imageName

	ext := filepath.Ext(tgt)
	if ext != schema.ACIExtension {
		errStr = fmt.Sprintf("build: Extension must be %s (given %s)", schema.ACIExtension, ext)
		errRes = errors.New(errStr)
		return errRes
	}

	if err := aci.ValidateLayout(root); err != nil {
		if e, ok := err.(aci.ErrOldVersion); ok {
			if debugEnabled {
				log.Printf("build: Warning: %v. Please update your manifest.", e)
			}
		} else {
			errStr = fmt.Sprintf("build: Layout failed validation: %v", err)
			errRes = errors.New(errStr)
			return errRes
		}
	}

	mode := os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	fh, err := os.OpenFile(tgt, mode, 0644)
	if err != nil {
		errStr = fmt.Sprintf("build: Unable to open target %s: %v", tgt, err)
		errRes = errors.New(errStr)
		return errRes
	}

	var gw *gzip.Writer
	var r io.WriteCloser = fh
	if !buildNocompress {
		gw = gzip.NewWriter(fh)
		r = gw
	}
	tr := tar.NewWriter(r)

	defer func() {
		tr.Close()
		if !buildNocompress {
			gw.Close()
		}
		fh.Close()
	}()

	mpath := filepath.Join(root, aci.ManifestFile)
	b, err := ioutil.ReadFile(mpath)
	if err != nil {
		errStr = fmt.Sprintf("build: Unable to read Image Manifest: %v", err)
		errRes = errors.New(errStr)
		return errRes
	}
	var im schema.ImageManifest
	if err := im.UnmarshalJSON(b); err != nil {
		errStr = fmt.Sprintf("build: Unable to load Image Manifest: %v", err)
		errRes = errors.New(errStr)
		return errRes
	}
	iw := aci.NewImageWriter(im, tr)

	err = filepath.Walk(root, aci.BuildWalker(root, iw))
	if err != nil {
		errStr = fmt.Sprintf("build: Error walking rootfs: %v", err)
		errRes = errors.New(errStr)
		return errRes
	}

	err = iw.Close()
	if err != nil {
		errStr = fmt.Sprintf("build: Unable to close image %s: %v", tgt, err)
		errRes = errors.New(errStr)
		return errRes
	}

	return nil
}
