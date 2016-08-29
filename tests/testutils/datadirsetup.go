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

package testutils

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/coreos/rkt/pkg/group"
	"github.com/hashicorp/errwrap"
)

const casrefDbPerm = os.FileMode(0660)
const boltDbPerm = os.FileMode(0660)

var (
	// dirs relative to data directory
	dirs = map[string]os.FileMode{
		".":   os.FileMode(0750 | os.ModeSetgid),
		"tmp": os.FileMode(0750 | os.ModeSetgid),

		// Cas directories.
		// Please keep in sync with dist/init/systemd/tmpfiles.d/rkt.conf
		// Make sure 'rkt' group can read/write some of the 'cas'
		// directories so that users in the group can fetch images
		"casref":           os.FileMode(0770 | os.ModeSetgid),
		"casref/blobdb":    os.FileMode(0770 | os.ModeSetgid),
		"casref/refdb":     os.FileMode(0770 | os.ModeSetgid),
		"casref/blob":      os.FileMode(0770 | os.ModeSetgid),
		"casref/bloblocks": os.FileMode(0770 | os.ModeSetgid),
		"casref/tmp":       os.FileMode(0770 | os.ModeSetgid),
		"treestore":        os.FileMode(0770 | os.ModeSetgid),
		"treestore/db":     os.FileMode(0770 | os.ModeSetgid),
		"treestore/tree":   os.FileMode(0700 | os.ModeSetgid),
		"treestore/locks":  os.FileMode(0700 | os.ModeSetgid),
		"locks":            os.FileMode(0750 | os.ModeSetgid),

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
		return errwrap.Wrap(fmt.Errorf("error setting %q directory group", path), err)
	}

	if err := os.Chmod(path, perm); err != nil {
		return errwrap.Wrap(fmt.Errorf("error setting %q directory permissions", path), err)
	}

	return nil
}

func createDirStructure(dataDir string, gid int) error {
	for dir, perm := range dirs {
		path := filepath.Join(dataDir, dir)

		if err := os.MkdirAll(path, perm); err != nil {
			return errwrap.Wrap(fmt.Errorf("error creating %q directory", path), err)
		}

		if err := setPermissions(path, 0, gid, perm); err != nil {
			return err
		}
	}

	return nil
}

func setupDataDir(dataDir string) error {
	gid, err := group.LookupGid("rkt")
	if err != nil {
		return err
	}

	if err := createDirStructure(dataDir, gid); err != nil {
		return err
	}

	if err := createFileWithPermissions(filepath.Join(dataDir, "casref", "blobdb", "db"), 0, gid, boltDbPerm); err != nil {
		return err
	}

	if err := createFileWithPermissions(filepath.Join(dataDir, "casref", "refdb", "db"), 0, gid, boltDbPerm); err != nil {
		return err
	}

	if err := createFileWithPermissions(filepath.Join(dataDir, "treestore", "db", "db"), 0, gid, boltDbPerm); err != nil {
		return err
	}

	return nil
}
