// Copyright 2014-2015 CoreOS, Inc.
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
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/appc/spec/schema/types"
	"github.com/coreos/rocket/pkg/lock"
)

// getContainers returns a slice representing the containers in the given rocket directory
func getContainers() ([]string, error) {
	cdir := containersDir()
	ls, err := ioutil.ReadDir(cdir)
	if err != nil {
		return nil, fmt.Errorf("cannot read containers directory: %v", err)
	}
	var cs []string
	for _, dir := range ls {
		if !dir.IsDir() {
			fmt.Fprintf(os.Stderr, "Unrecognized file: %q, ignoring", dir)
			continue
		}
		cs = append(cs, dir.Name())
	}
	return cs, nil
}

// getContainerLockAndState opens the container directory in the form of a lock.DirLock,
// returning the lock and wether the container has already exited or not.
func getContainerLockAndState(containerUUID *types.UUID) (l *lock.DirLock, isExited bool, err error) {
	cid := containerUUID.String()
	isGarbage := false

	cp := filepath.Join(containersDir(), cid)
	l, err = lock.NewLock(cp)
	if err == lock.ErrNotExist {
		// Fallback to garbage/$cid if containers/$cid is missing, "rkt gc" renames exited containers to garbage/$cid.
		isGarbage = true
		cp = filepath.Join(garbageDir(), cid)
		l, err = lock.NewLock(cp)
	}

	if err != nil {
		if err == lock.ErrNotExist {
			err = fmt.Errorf("container %v not found", cid)
		} else {
			err = fmt.Errorf("error opening lock: %v", err)
		}
		return
	}

	isExited = true
	if flagWait && !isGarbage {
		err = l.SharedLock()
	} else {
		err = l.TrySharedLock()
		if err == lock.ErrLocked {
			if isGarbage {
				// Container is exited and being deleted, we can't reliably query its status, it's effectively gone.
				err = fmt.Errorf("unable to query status: %q is being removed", cid)
				return
			}
			isExited = false
			err = nil
		}
	}

	if err != nil {
		err = fmt.Errorf("error acquiring lock: %v", err)
	}

	return
}
