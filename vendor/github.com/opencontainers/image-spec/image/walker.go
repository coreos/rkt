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
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

var (
	errEOW = fmt.Errorf("end of walk") // error to signal stop walking
)

// walkFunc is a function type that gets called for each file or directory visited by the Walker.
type walkFunc func(path string, _ os.FileInfo, _ io.Reader) error

// walker is the interface that walks through a file tree,
// calling walk for each file or directory in the tree.
type walker interface {
	walk(walkFunc) error
}

type tarWalker struct {
	r io.ReadSeeker
}

// newTarWalker returns a Walker that walks through .tar files.
func newTarWalker(r io.ReadSeeker) walker {
	return &tarWalker{r}
}

func (w *tarWalker) walk(f walkFunc) error {
	if _, err := w.r.Seek(0, os.SEEK_SET); err != nil {
		return errors.Wrapf(err, "unable to reset")
	}

	tr := tar.NewReader(w.r)

loop:
	for {
		hdr, err := tr.Next()
		switch err {
		case io.EOF:
			break loop
		case nil:
			// success, continue below
		default:
			return errors.Wrapf(err, "error advancing tar stream")
		}

		info := hdr.FileInfo()
		if err := f(hdr.Name, info, tr); err != nil {
			return err
		}
	}

	return nil
}

type eofReader struct{}

func (eofReader) Read(_ []byte) (int, error) {
	return 0, io.EOF
}

type pathWalker struct {
	root string
}

// newPathWalker returns a Walker that walks through directories
// starting at the given root path. It does not follow symlinks.
func newPathWalker(root string) walker {
	return &pathWalker{root}
}

func (w *pathWalker) walk(f walkFunc) error {
	return filepath.Walk(w.root, func(path string, info os.FileInfo, err error) error {
		// MUST check error value, to make sure the `os.FileInfo` is available.
		// Otherwise panic risk will exist.
		if err != nil {
			return errors.Wrap(err, "error walking path")
		}

		rel, err := filepath.Rel(w.root, path)
		if err != nil {
			return errors.Wrap(err, "error walking path") // err from filepath.Walk includes path name
		}

		if info.IsDir() { // behave like a tar reader for directories
			return f(rel, info, eofReader{})
		}

		file, err := os.Open(path)
		if err != nil {
			return errors.Wrap(err, "unable to open file") // os.Open includes the path
		}
		defer file.Close()

		return f(rel, info, file)
	})
}
