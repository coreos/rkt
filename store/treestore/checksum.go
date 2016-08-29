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
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"sort"

	specaci "github.com/appc/spec/aci"
	"github.com/appc/spec/pkg/tarheader"
)

type xattr struct {
	Name  string
	Value string
}

// Like tar Header but, to keep json output reproducible:
// * Xattrs as a slice
// * Skip Uname and Gname
// TODO. Should ModTime/AccessTime/ChangeTime be saved? For validation its
// probably enough to hash the file contents and the other infos and avoid
// problems due to them changing.
// TODO(sgotti) Is it possible that json output will change between go
// versions? Use another or our own Marshaller?
type fileInfo struct {
	Name     string // name of header file entry
	Mode     int64  // permission and mode bits
	Uid      int    // user id of owner
	Gid      int    // group id of owner
	Size     int64  // length in bytes
	Typeflag byte   // type of header entry
	Linkname string // target name of link
	Devmajor int64  // major number of character or block device
	Devminor int64  // minor number of character or block device
	Xattrs   []xattr
}

func fileInfoFromHeader(hdr *tar.Header) *fileInfo {
	fi := &fileInfo{
		Name:     hdr.Name,
		Mode:     hdr.Mode,
		Uid:      hdr.Uid,
		Gid:      hdr.Gid,
		Size:     hdr.Size,
		Typeflag: hdr.Typeflag,
		Linkname: hdr.Linkname,
		Devmajor: hdr.Devmajor,
		Devminor: hdr.Devminor,
	}
	keys := make([]string, 0, len(hdr.Xattrs))
	for k := range hdr.Xattrs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	xattrs := make([]xattr, 0, len(keys))
	for _, k := range keys {
		xattrs = append(xattrs, xattr{Name: k, Value: hdr.Xattrs[k]})
	}
	fi.Xattrs = xattrs
	return fi
}

// TODO(sgotti) this func is copied from appcs/spec/aci/build.go but also
// removes the hash, rendered and image files. Find a way to reuse it.
func buildWalker(root string, aw specaci.ArchiveWriter) filepath.WalkFunc {
	// cache of inode -> filepath, used to leverage hard links in the archive
	inos := map[uint64]string{}
	return func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relpath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if relpath == "." {
			return nil
		}
		if relpath == specaci.ManifestFile {
			// ignore; this will be written by the archive writer
			// TODO(jonboulle): does this make sense? maybe just remove from archivewriter?
			return nil
		}

		link := ""
		var r io.Reader
		switch info.Mode() & os.ModeType {
		case os.ModeSocket:
			return nil
		case os.ModeNamedPipe:
		case os.ModeCharDevice:
		case os.ModeDevice:
		case os.ModeDir:
		case os.ModeSymlink:
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			link = target
		default:
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()
			r = file
		}

		hdr, err := tar.FileInfoHeader(info, link)
		if err != nil {
			panic(err)
		}
		// Because os.FileInfo's Name method returns only the base
		// name of the file it describes, it may be necessary to
		// modify the Name field of the returned header to provide the
		// full path name of the file.
		hdr.Name = relpath
		tarheader.Populate(hdr, info, inos)
		// If the file is a hard link to a file we've already seen, we
		// don't need the contents
		if hdr.Typeflag == tar.TypeLink {
			hdr.Size = 0
			r = nil
		}

		return aw.AddFile(hdr, r)
	}
}

type imageHashWriter struct {
	io.Writer
}

func newHashWriter(w io.Writer) specaci.ArchiveWriter {
	return &imageHashWriter{w}
}

func (aw *imageHashWriter) AddFile(hdr *tar.Header, r io.Reader) error {
	// Write the json encoding of the FileInfo struct
	hdrj, err := json.Marshal(fileInfoFromHeader(hdr))
	if err != nil {
		return err
	}
	_, err = aw.Writer.Write(hdrj)
	if err != nil {
		return err
	}

	if r != nil {
		// Write the file data
		_, err := io.Copy(aw.Writer, r)
		if err != nil {
			return err
		}
	}

	return nil
}

func (aw *imageHashWriter) Close() error {
	return nil
}

func hashString(h hash.Hash) string {
	s := h.Sum(nil)
	return fmt.Sprintf("%s%x", hashPrefix, s)
}
