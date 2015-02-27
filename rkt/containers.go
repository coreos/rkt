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

//+build linux

package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"time"

	"github.com/coreos/rocket/Godeps/_workspace/src/code.google.com/p/go-uuid/uuid"
	"github.com/coreos/rocket/Godeps/_workspace/src/github.com/appc/spec/schema/types"
	"github.com/coreos/rocket/pkg/lock"
)

//
// UUID's are allocated by creating a name in a directory:
// 0. allocate uuid	containers/uuid/$uuid
//
// This uuid placeholder exists until after the respective container's directory
// is deleted.
//
// Checking if a UUID is in use is a matter of `test -e // containers/uuid/$uuid`
//
// Container directories progress monotonically through the following states:
// 0. embryonic		containers/embryos/$uuid
// 1. preparing		containers/prepared/$uuid & locked exclusively
// 2. prepared		containers/prepared/$uuid
// 3. running		containers/containers/$uuid & locked exclusively
// 4. exited		containers/containers/$uuid
// 5. garbage		containers/garbage/$uuid
// 6. deleting		containers/garbage/$uuid + locked exclusively
//
// Whenever a container directory migrates across state directories, it does so
// atomically via rename().
//
// This strategy permits lock-free location of a container's directory by UUID
// by performing the lookups into the individual state directories in the same
// order as the states progress.  The state a container is in may be inferred
// from which directory it was opened within, and whether that directory is
// locked exclusively (for preparing, exited, and deleting states).
//
// A container's state may change during a getContainer() or walkContainers()
// operation, thus the state returned by getContaienr or iterated on by
// walkContainers() is instantaneous and may be stale immediately.  The
// container structure contains the opened directory handle which may be used
// for *At()-based opening of descendant names in the filesystem, which
// functions correctly despite the container's directory being potentially
// renamed since opening.  This is necessary because deriving a path to the
// container's directory from its supposed state would be unreliable.
//
// walkContainers() supports filtering based on container state, care must be
// taken to ensure the callback is only invoked for containers in one of the
// desired states.
//
// getContainer() used by walkContainers() will look for the container by uuid
// in all state directories.  Even if only the desired state directories were
// included in the listing process, we may still get a container which has
// migrated in the mean time into an unrequested state.  For this reason the
// result of getContainer() is checked against the filter before invoking the
// callback.  Additionally, though the per-state lists are acquired in the same
// order as a container's state progresses, there is potential for the same
// container to become listed twice; if a container migrates between states
// simultaneous to walkContainers() transition between the states in listing the
// containers.  For this reason, the aggregated list is uniqued before walking.
//
// TODO(vc): update Documentation/container-lifecycle.md to reflect this once
// reviewed

type container struct {
	*lock.DirLock
	uuid        *types.UUID
	createdByMe bool // true if we're the creator of this container (only the creator can xToPrepare or xToRun directly from preparing)

	isEmbryo    bool // container directories start as embryos before xToPreparing(): (lock(containers/embryo/$uuid) && rename -> containers/prepared/$uuid)
	isPreparing bool // when locked at containers/prepared/$uuid the container is preparing
	isPrepared  bool // when unlocked at containers/prepared/$uuid the container is prepared
	isExited    bool // when locked at containers/run/$uuid the container is running, when not locked here it's exited
	isGarbage   bool // when unlocked at containers/garbage/$uuid the container is exited and is garbage
	isDeleting  bool // when locked at containers/garbage/$uuid the container is exited, garbage, and being actively deleted
	isGone      bool // when not found anywhere
}

type includeMask byte

const (
	includeEmbryoDir includeMask = 1 << iota
	includePreparedDir
	includeRunDir
	includeGarbageDir

	includeMostDirs includeMask = (includeRunDir | includeGarbageDir | includePreparedDir)
	includeAllDirs  includeMask = (includeMostDirs | includeEmbryoDir)
)

var (
	containersInitialized = false
)

// initContainers creates the required global directories
func initContainers() error {
	if !containersInitialized {
		dirs := []string{uuidDir(), embryoDir(), preparedDir(), runDir(), garbageDir()}
		for _, d := range dirs {
			if err := os.MkdirAll(d, 0700); err != nil {
				return fmt.Errorf("error creating directory: %v", err)
			}
		}
		containersInitialized = true
	}
	return nil
}

// walkContainers iterates over the included directories calling function f for every container found.
func walkContainers(include includeMask, f func(*container)) error {
	if err := initContainers(); err != nil {
		return err
	}

	ls, err := listContainers(include)
	if err != nil {
		return fmt.Errorf("failed to get containers: %v", err)
	}
	sort.Strings(ls)

	for _, uuid := range ls {
		c, err := getContainer(uuid)
		if err != nil {
			stderr("Skipping %q: %v", uuid, err)
			continue
		}

		// omit containers found in unrequested states
		// this is to cover a race between listContainers finding the uuids and container states changing
		// it's preferable to keep these operations lock-free, for example a `rkt gc` shouldn't block `rkt run`.
		if c.isEmbryo && include&includeEmbryoDir == 0 ||
			c.isGarbage && include&includeGarbageDir == 0 ||
			((c.isPrepared || c.isPreparing) && include&includePreparedDir == 0) ||
			!c.isGarbage && !c.isPreparing && !c.isPrepared && include&includeRunDir == 0 {
			c.Close()
			continue
		}

		f(c)
		c.Close()
	}

	return nil
}

// newContainer creates a new container directory in the "preparing" state, allocating a unique uuid for it in the process.
// The returned container is always left in an exclusively locked state (preparing is locked in the prepared directory)
// The container must be closed using container.Close()
func newContainer() (*container, error) {
	if err := initContainers(); err != nil {
		return nil, err
	}

	c := &container{
		createdByMe: true,
		isEmbryo:    true, // starts as an embryo, then xToPreparing locks, renames, and sets isPreparing
		isPreparing: false,
		isPrepared:  false,
		isExited:    false,
		isGarbage:   false,
		isDeleting:  false,
	}

	for i := 0; i < 100; i++ { // prevent potential for an infinite loop
		var err error
		c.uuid, err = types.NewUUID(uuid.New())
		if err != nil {
			return nil, fmt.Errorf("error creating UUID: %v", err)
		}

		err = os.Mkdir(c.uuidPath(), 0700)
		if os.IsExist(err) {
			continue
		} else if err != nil {
			return nil, err
		}

		err = os.Mkdir(c.embryoPath(), 0700)
		if err != nil {
			os.Remove(c.uuidPath())
			return nil, err
		}

		c.DirLock, err = lock.NewLock(c.embryoPath())
		if err != nil {
			os.Remove(c.uuidPath())
			os.Remove(c.embryoPath())
			return nil, err
		}

		err = c.xToPreparing()
		if err != nil {
			return nil, err
		}

		// At this point we we have:
		// /var/lib/rkt/containers/uuid/$uuid
		// /var/lib/rkt/containers/prepared/$uuid << exclusively locked to indicate "preparing"

		return c, nil
	}

	return nil, fmt.Errorf("exhausted uuid allocation attempts")
}

// getContainer returns a container struct representing the given container.
// The returned lock is always left in an open but unlocked state.
// The container must be closed using container.Close()
func getContainer(uuid string) (*container, error) {
	if err := initContainers(); err != nil {
		return nil, err
	}

	c := &container{
		isEmbryo:    false,
		isPreparing: false,
		isPrepared:  false,
		isExited:    false,
		isGarbage:   false,
		isDeleting:  false,
	}

	u, err := types.NewUUID(uuid)
	if err != nil {
		return nil, err
	}
	c.uuid = u

	// we try open the container in all possible directories, in the same order the states occur
	l, err := lock.NewLock(c.embryoPath())
	if err == nil {
		c.isEmbryo = true
	} else if err == lock.ErrNotExist {
		l, err = lock.NewLock(c.preparedPath())
		if err == nil {
			c.isPrepared = true
		} else if err == lock.ErrNotExist {
			l, err = lock.NewLock(c.runPath())
			if err == lock.ErrNotExist {
				l, err = lock.NewLock(c.garbagePath())
				if err == lock.ErrNotExist {
					c.isGone = true
				} else if err == nil {
					c.isGarbage = true
					c.isExited = true // garbage is _always_ implicitly exited
				}
			}
		}

		// prepared, run, and garbage states use exclusive locks to indicate preparing, running, and deleting
		if err = l.TrySharedLock(); err != nil {
			if err != lock.ErrLocked {
				l.Close()
				return nil, fmt.Errorf("unexpected lock error: %v", err)
			}
			if c.isGarbage {
				c.isDeleting = true
			} else if c.isPrepared {
				c.isPrepared = false
				c.isPreparing = true
			}
			err = nil
		} else {
			l.Unlock()
			c.isExited = true // idempotent WRT isGarbage case.
		}
	}

	if err != nil {
		return nil, fmt.Errorf("error opening container %q: %v", uuid, err)
	}

	c.DirLock = l

	return c, nil
}

// path returns the path to the container according to the current (cached) state
func (c *container) path() string {
	if c.isEmbryo {
		return c.embryoPath()
	} else if c.isPreparing || c.isPrepared {
		return c.preparedPath()
	} else if c.isGarbage {
		return c.garbagePath()
	} else if c.isGone {
		return ""
	}

	return c.runPath()
}

// uuidPath returns the path to the container's uuid placeholder
func (c *container) uuidPath() string {
	return filepath.Join(uuidDir(), c.uuid.String())
}

// embryoPath returns the path to the container where it would be in the embryoDir in its embryonic (pre-locked) state.
func (c *container) embryoPath() string {
	return filepath.Join(embryoDir(), c.uuid.String())
}

// preparedPath returns the path to the container where it would be in the preparedDir.
func (c *container) preparedPath() string {
	return filepath.Join(preparedDir(), c.uuid.String())
}

// runPath returns the path to the container where it would be in the runDir.
func (c *container) runPath() string {
	return filepath.Join(runDir(), c.uuid.String())
}

// garbagePath returns the path to the container where it would be in the garbageDir.
func (c *container) garbagePath() string {
	return filepath.Join(garbageDir(), c.uuid.String())
}

// xToPreparing transitions a container from embryo -> preparing, leaves the container locked
func (c *container) xToPreparing() error {
	if !c.createdByMe {
		return fmt.Errorf("bug: only containers created by me may transition to preparing")
	}

	if !c.isEmbryo {
		return fmt.Errorf("bug: only embryonic containers can transition to preparing")
	}

	if err := c.ExclusiveLock(); err != nil {
		return err
	}

	if err := os.Rename(c.embryoPath(), c.preparedPath()); err != nil {
		return err
	}

	c.isEmbryo = false
	c.isPreparing = true

	return nil
}

// xToPrepared transitions a container from preparing -> prepared
func (c *container) xToPrepared() error {
	if !c.createdByMe {
		return fmt.Errorf("bug: only containers created by me may transition to prepared")
	}

	if !c.isPreparing {
		return fmt.Errorf("bug: only preparing containers may transition to prepared")
	}

	if err := c.Unlock(); err != nil {
		return err
	}

	c.isPreparing = false
	c.isPrepared = true

	return nil
}

// xToRun transitions a container from prepared -> run (or if created by me, may leap from preparing -> run)
func (c *container) xToRun() error {
	if !c.createdByMe && !c.isPrepared {
		return fmt.Errorf("bug: only prepared containers may transition to run externally")
	}

	if c.createdByMe && !c.isPrepared && !c.isPreparing {
		return fmt.Errorf("bug: only prepared or preparing containers may transition to run")
	}

	if err := c.ExclusiveLock(); err != nil {
		return err
	}

	if err := os.Rename(c.preparedPath(), c.runPath()); err != nil {
		return err
	}

	c.isPreparing = false
	c.isPrepared = false

	return nil
}

// xToGarbage transitions a container from run -> garbage
func (c *container) xToGarbage() error {
	if !c.isExited {
		return fmt.Errorf("bug: only exited containers may transition to garbage")
	}

	if err := os.Rename(c.runPath(), c.garbagePath()); err != nil {
		return err
	}

	c.isGarbage = true

	return nil
}

// listContainers returns a list of container uuids in string form.
func listContainers(include includeMask) ([]string, error) {
	// uniqued due to the possibility of a container being renamed from across directories during the list operation
	ucs := make(map[string]struct{})
	dirs := []struct {
		kind includeMask
		path string
	}{
		{ // the order here is significant: embryo -> prepared -> running -> garbage
			kind: includeEmbryoDir,
			path: embryoDir(),
		}, {
			kind: includePreparedDir,
			path: preparedDir(),
		}, {
			kind: includeRunDir,
			path: runDir(),
		}, {
			kind: includeGarbageDir,
			path: garbageDir(),
		},
	}

	for _, d := range dirs {
		if include&d.kind != 0 {
			cs, err := listContainersFromDir(d.path)
			if err != nil {
				return nil, err
			}
			for _, c := range cs {
				ucs[c] = struct{}{}
			}
		}
	}

	cs := make([]string, 0, len(ucs))
	for c := range ucs {
		cs = append(cs, c)
	}

	return cs, nil
}

// listContainersFromDir returns a list of container uuids in string form from a specific directory.
func listContainersFromDir(cdir string) ([]string, error) {
	var cs []string

	ls, err := ioutil.ReadDir(cdir)
	if err != nil {
		if os.IsNotExist(err) {
			return cs, nil
		}
		return nil, fmt.Errorf("cannot read containers directory: %v", err)
	}

	for _, c := range ls {
		if !c.IsDir() {
			stderr("Unrecognized entry: %q, ignoring", c.Name())
			continue
		}
		cs = append(cs, c.Name())
	}

	return cs, nil
}

// refreshState() updates the cached members of c to reflect current reality
func (c *container) refreshState() error {
	//  TODO(vc): this overlaps substantially with newContainer(), could probably unify.
	c.isEmbryo = false
	c.isPreparing = false
	c.isPrepared = false
	c.isExited = false
	c.isGarbage = false
	c.isDeleting = false
	c.isGone = false

	// we try find the container in all possible directories, in the same order the states occur
	_, err := os.Stat(c.embryoPath())
	if err == nil {
		c.isEmbryo = true
	} else if os.IsNotExist(err) {
		_, err = os.Stat(c.preparedPath())
		if err == nil {
			c.isPrepared = true
		} else if os.IsNotExist(err) {
			_, err = os.Stat(c.runPath())
			if os.IsNotExist(err) {
				_, err = os.Stat(c.garbagePath())
				if os.IsNotExist(err) {
					c.isGone = true
				} else if err == nil {
					c.isGarbage = true
					c.isExited = true // garbage is _always_ implicitly exited
				}
			}
		}

		// prepared, run, and garbage states use exclusive locks to indicate preparing, running, and deleting
		if err = c.TrySharedLock(); err != nil {
			if err == lock.ErrLocked {
				if c.isGarbage {
					c.isDeleting = true
				} else if c.isPrepared {
					c.isPrepared = false
					c.isPreparing = true
				}
				err = nil
			}
		} else {
			c.Unlock()
			c.isExited = true // covers runPath() exited containers, idempotent WRT isGarbage case.
		}
	}

	if err != nil {
		return fmt.Errorf("error refreshing state of %q: %v", c.uuid.String(), err)
	}

	return nil
}

// waitExited waits for a container to (run) and exit.
func (c *container) waitExited() error {
	for c.isPreparing || c.isPrepared || !c.isExited {
		if err := c.SharedLock(); err != nil {
			return err
		}

		if err := c.Unlock(); err != nil {
			return err
		}

		if err := c.refreshState(); err != nil {
			return err
		}

		// if we're in the gap between preparing and running in a split prepare/run-prepared usage, take a nap
		if c.isPrepared {
			time.Sleep(time.Second)
		}
	}
	return nil
}

// readFile reads an entire file from a container's directory.
func (c *container) readFile(path string) ([]byte, error) {
	f, err := c.openFile(path, syscall.O_RDONLY)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return ioutil.ReadAll(f)
}

// readIntFromFile reads an int from a file in a container's directory.
func (c *container) readIntFromFile(path string) (i int, err error) {
	b, err := c.readFile(path)
	if err != nil {
		return
	}
	_, err = fmt.Sscanf(string(b), "%d", &i)
	return
}

// openFile opens a file from a container's directory returning a file descriptor.
func (c *container) openFile(path string, flags int) (*os.File, error) {
	cdirfd, err := c.Fd()
	if err != nil {
		return nil, err
	}

	fd, err := syscall.Openat(cdirfd, path, flags, 0)
	if err != nil {
		return nil, fmt.Errorf("unable to open file: %v", err)
	}

	return os.NewFile(uintptr(fd), path), nil
}

// getState returns the current state of the container
func (c *container) getState() string {
	state := "running"
	if c.isEmbryo {
		state = "embryo"
	} else if c.isPreparing {
		state = "preparing"
	} else if c.isPrepared {
		state = "prepared"
	} else if c.isExited { // this covers c.isGarbage
		state = "exited"
	} else if c.isDeleting {
		state = "deleting"
	} else if c.isGone {
		state = "gone"
	}

	return state
}

// getPID returns the pid of the container.
func (c *container) getPID() (int, error) {
	return c.readIntFromFile("pid")
}

// getExitStatuses returns a map of the statuses of the container.
func (c *container) getExitStatuses() (map[string]int, error) {
	sdir, err := c.openFile(statusDir, syscall.O_RDONLY|syscall.O_DIRECTORY)
	if err != nil {
		return nil, fmt.Errorf("unable to open status directory: %v", err)
	}
	defer sdir.Close()

	ls, err := sdir.Readdirnames(0)
	if err != nil {
		return nil, fmt.Errorf("unable to read status directory: %v", err)
	}

	stats := make(map[string]int)
	for _, name := range ls {
		s, err := c.readIntFromFile(filepath.Join(statusDir, name))
		if err != nil {
			stderr("Unable to get status of app %q: %v", name, err)
			continue
		}
		stats[name] = s
	}
	return stats, nil
}
