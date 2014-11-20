package raf

// Package raf provides functionality to validate applications implementing the
// Runnable App Format, by verifying an on-disk layout

/*

Filesystem Layout

The on-disk format is straightforward, with a rootfs and an app manifest.

/manifest
/rootfs
/rootfs/usr/bin/mysql
/rootfs/....

*/

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/containers/standard/schema"
	"github.com/containers/standard/schema/types"
)

var (
	ErrNoRootFS      = errors.New("no rootfs found in layout")
	ErrNoAppManifest = errors.New("no app manifest found in layout")
)

// ValidateRAF takes a directory and validates that the layout of the directory
// matches that expected by the Runnable App Format.
// If any errors are encountered during the validation, it will abort and
// return the first one.
// TODO(jonboulle): work on tar streams instead of requiring this be on disk?
func ValidateRAF(dir string) error {
	fi, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("error accessing layout: %v", err)
	}
	if !fi.IsDir() {
		return fmt.Errorf("given path %q is not a directory", dir)
	}
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("error reading directory: %v", err)
	}
	var rootfs, manifest string
	for _, f := range files {
		name := f.Name()
		fpath := filepath.Join(dir, name)
		switch name {
		case "manifest":
			manifest = fpath
		case "rootfs":
			rootfs = fpath
		default:
			log.Printf("unrecognized file path %q - ignoring", fpath)
		}
	}
	if manifest == "" {
		return ErrNoAppManifest
	}
	if rootfs == "" {
		return ErrNoRootFS
	}
	rfsfiles, err := validateAppManifest(manifest)
	if err != nil {
		return err
	}
	if err := validateRootfs(rootfs, rfsfiles); err != nil {
		return err
	}
	return nil
}

// validateAppManifest ensures that the file at the given path is a valid
// AppManifest. It returns a map of all files described in the manifest.
func validateAppManifest(fpath string) (map[string]types.File, error) {
	f, err := os.Open(fpath)
	if err != nil {
		return nil, fmt.Errorf("error opening app manifest: %v", err)
	}
	b, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("error reading app manifest: %v", err)
	}
	var am schema.AppManifest
	if err = json.Unmarshal(b, &am); err != nil {
		return nil, fmt.Errorf("error unmarshaling app manifest: %v", err)
	}
	return am.Files, nil
}

// validateRootfs ensures that a given rootfs filesystem is valid.
func validateRootfs(dir string, files map[string]types.File) error {
	fi, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("error accessing rootfs: %v", err)
	}
	if !fi.IsDir() {
		return fmt.Errorf("rootfs is not a directory")
	}
	for path, file := range files {
		if err := validateFile(path, file); err != nil {
			return err
		}
	}
	return nil
}

func validateFile(path string, file types.File) error {
	// TODO(jonboulle): implement me..
	return nil
}
