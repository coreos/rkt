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

package main

import (
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/spf13/cobra"
	"github.com/coreos/rkt/common"
	"github.com/coreos/rkt/store"
)

const casDbPerm = os.FileMode(0660)

var (
	cmdInstall = &cobra.Command{
		Use:   "install",
		Short: "Set up rkt data directories with correct permissions",
		Run:   runWrapper(runInstall),
	}

	// dirs relative to globalFlags.Dir
	dirs = map[string]os.FileMode{
		".":   os.FileMode(0750 | os.ModeSetgid),
		"tmp": os.FileMode(0750 | os.ModeSetgid),

		// Cas directories.
		// Make sure 'rkt' group can read/write some of the 'cas'
		// directories so that users in the group can fetch images
		"cas":                os.FileMode(0770 | os.ModeSetgid),
		"cas/db":             os.FileMode(0770 | os.ModeSetgid),
		"cas/imagelocks":     os.FileMode(0770 | os.ModeSetgid),
		"cas/imageManifest":  os.FileMode(0770 | os.ModeSetgid),
		"cas/blob":           os.FileMode(0770 | os.ModeSetgid),
		"cas/tmp":            os.FileMode(0770 | os.ModeSetgid),
		"cas/tree":           os.FileMode(0700 | os.ModeSetgid),
		"cas/treestorelocks": os.FileMode(0700 | os.ModeSetgid),
		"locks":              os.FileMode(0750 | os.ModeSetgid),

		// Pods directories.
		"pods":                os.FileMode(0750 | os.ModeSetgid),
		"pods/embryo":         os.FileMode(0750 | os.ModeSetgid),
		"pods/prepare":        os.FileMode(0750 | os.ModeSetgid),
		"pods/prepared":       os.FileMode(0750 | os.ModeSetgid),
		"pods/run":            os.FileMode(0750 | os.ModeSetgid),
		"pods/exited-garbage": os.FileMode(0750 | os.ModeSetgid),
		"pods/garbage":        os.FileMode(0750 | os.ModeSetgid),
	}
)

func init() {
	cmdRkt.AddCommand(cmdInstall)
}

func createFileWithPermissions(path string, uid int, gid int, perm os.FileMode) error {
	_, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0666)
	if err != nil {
		if !os.IsExist(err) {
			return err
		}
		// file exists
	}

	return setPermissions(path, uid, gid, perm)
}

func setPermissions(path string, uid int, gid int, perm os.FileMode) error {
	if err := os.Chown(path, uid, gid); err != nil {
		return fmt.Errorf("error setting %q directory group: %v", path, err)
	}

	if err := os.Chmod(path, perm); err != nil {
		return fmt.Errorf("error setting %q directory permissions: %v", path, err)
	}

	return nil
}

func createDirStructure(gid int) error {
	for dir, perm := range dirs {
		path := filepath.Join(globalFlags.Dir, dir)

		if err := os.MkdirAll(path, perm); err != nil {
			return fmt.Errorf("error creating %q directory: %v", path, err)
		}

		if err := setPermissions(path, 0, gid, perm); err != nil {
			return err
		}
	}

	return nil
}

func setCasDbFilesPermissions(casDbPath string, gid int, perm os.FileMode) error {
	casDbWalker := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			if err := setPermissions(path, 0, gid, perm); err != nil {
				return err
			}
		}

		return nil
	}

	if err := filepath.Walk(casDbPath, casDbWalker); err != nil {
		return err
	}

	return nil
}

func createDbFiles(casDbPath string, gid int, perm os.FileMode) error {
	dbPath := filepath.Join(casDbPath, store.DbFilename)
	if err := createFileWithPermissions(dbPath, 0, gid, perm); err != nil {
		return fmt.Errorf("error creating %s: %v", dbPath, err)
	}

	// ql database uses a Write-Ahead Logging (WAL) file whose name is
	// generated from the sha1 hash of the database name
	h := sha1.New()
	io.WriteString(h, store.DbFilename)
	walFilename := fmt.Sprintf(".%x", h.Sum(nil))
	walFilePath := filepath.Join(casDbPath, walFilename)
	if err := createFileWithPermissions(walFilePath, 0, gid, perm); err != nil {
		return fmt.Errorf("error creating %s: %v", walFilename, err)
	}

	return nil
}

func runInstall(cmd *cobra.Command, args []string) (exit int) {
	gid, err := common.LookupGid(common.RktGroup)
	if err != nil {
		stderr("install: error looking up rkt gid: %v", err)
		return 1
	}

	if err := createDirStructure(gid); err != nil {
		stderr("install: error creating rkt directory structure: %v", err)
		return 1
	}

	casDbPath := filepath.Join(globalFlags.Dir, "cas", "db")
	if err := setCasDbFilesPermissions(casDbPath, gid, casDbPerm); err != nil {
		stderr("install: error setting cas db permissions: %v", err)
		return 1
	}

	if err := createDbFiles(casDbPath, gid, casDbPerm); err != nil {
		stderr("install: error creating db files: %v", err)
		return 1
	}
	stderr("rkt directory structure successfully created.")

	return 0
}
