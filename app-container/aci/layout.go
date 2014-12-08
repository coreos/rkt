package aci

/*

Image Layout

The on-disk layout of an app container is straightforward.
It includes a rootfs with all of the files that will exist in the root of the app and a manifest describing the image.
The layout must contain an app image manifest.

/manifest
/rootfs/
/rootfs/usr/bin/mysql

*/

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/coreos/rocket/app-container/schema"
)

var (
	ErrNoRootFS   = errors.New("no rootfs found in layout")
	ErrNoManifest = errors.New("no app image manifest found in layout")
)

// ValidateLayout takes a directory and validates that the layout of the directory
// matches that expected by the Application Container Image format.
// If any errors are encountered during the validation, it will abort and
// return the first one.
func ValidateLayout(dir string) error {
	fi, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("error accessing layout: %v", err)
	}
	if !fi.IsDir() {
		return fmt.Errorf("given path %q is not a directory", dir)
	}
	var flist []string
	var amOK, rfsOK bool
	var am io.Reader
	walkLayout := func(fpath string, fi os.FileInfo, err error) error {
		rpath := strings.TrimPrefix(fpath, dir)
		name := filepath.Base(rpath)
		switch name {
		case ".":
		case "app":
			am, err = os.Open(fpath)
			if err != nil {
				return err
			}
			amOK = true
		case "rootfs":
			if !fi.IsDir() {
				return errors.New("rootfs is not a directory")
			}
			rfsOK = true
		default:
			flist = append(flist, rpath)
		}
		return nil
	}
	if err := filepath.Walk(dir, walkLayout); err != nil {
		return err
	}
	return validate(amOK, am, rfsOK, flist)
}

// ValidateLayout takes a *tar.Reader and validates that the layout of the
// filesystem the reader encapsulates matches that expected by the
// Application Container Image format.  If any errors are encountered during
// the validation, it will abort and return the first one.
func ValidateArchive(tr *tar.Reader) error {
	var flist []string
	var amOK, rfsOK bool
	var am bytes.Buffer
Tar:
	for {
		hdr, err := tr.Next()
		switch {
		case err == nil:
		case err == io.EOF:
			break Tar
		default:
			return err
		}
		switch hdr.Name {
		case "app":
			_, err := io.Copy(&am, tr)
			if err != nil {
				return err
			}
			amOK = true
		case "rootfs/":
			if !hdr.FileInfo().IsDir() {
				return fmt.Errorf("rootfs is not a directory")
			}
			rfsOK = true
		default:
			flist = append(flist, hdr.Name)
		}
	}
	return validate(amOK, &am, rfsOK, flist)
}

func validate(amOK bool, am io.Reader, rfsOK bool, files []string) error {
	if !amOK {
		return ErrNoManifest
	}
	if amOK {
		if !rfsOK {
			return ErrNoRootFS
		}
		b, err := ioutil.ReadAll(am)
		if err != nil {
			return fmt.Errorf("error reading app manifest: %v", err)
		}
		var a schema.AppImageManifest
		if err := a.UnmarshalJSON(b); err != nil {
			return fmt.Errorf("app manifest validation failed: %v", err)
		}
	}
	for _, f := range files {
		if !strings.HasPrefix(f, "rootfs") {
			return fmt.Errorf("unrecognized file path in layout: %q", f)
		}
	}
	return nil
}

// validateAppImageManifest ensures that the given io.Reader represents a valid
// AppImageManifest.
func validateAppImageManifest(r io.Reader) error {
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return fmt.Errorf("error reading app manifest: %v", err)
	}
	var am schema.AppImageManifest
	if err = json.Unmarshal(b, &am); err != nil {
		return fmt.Errorf("error unmarshaling app manifest: %v", err)
	}
	return nil
}
