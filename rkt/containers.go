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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"time"

	"github.com/coreos/rocket/Godeps/_workspace/src/code.google.com/p/go-uuid/uuid"
	"github.com/coreos/rocket/Godeps/_workspace/src/github.com/appc/spec/schema"
	"github.com/coreos/rocket/Godeps/_workspace/src/github.com/appc/spec/schema/types"
	"github.com/coreos/rocket/common"
	"github.com/coreos/rocket/networking/netinfo"
	"github.com/coreos/rocket/pkg/lock"
)

// see Documentation/container-lifecycle.md for some explanation

type container struct {
	*lock.FileLock
	uuid        *types.UUID
	createdByMe bool              // true if we're the creator of this container (only the creator can xToPrepare or xToRun directly from preparing)
	nets        []netinfo.NetInfo // list of networks (name, IP, iface) this container is using

	isEmbryo         bool // directory starts as embryo before entering preparing state, serves as stage for acquiring lock before rename to prepare/.
	isPreparing      bool // when locked at pods/prepare/$uuid the container is actively being prepared
	isAbortedPrepare bool // when unlocked at pods/prepare/$uuid the container never finished preparing
	isPrepared       bool // when at pods/prepared/$uuid the container is prepared, serves as stage for acquiring lock before rename to run/.
	isExited         bool // when locked at pods/run/$uuid the container is running, when unlocked it's exited.
	isExitedGarbage  bool // when unlocked at pods/exited-garbage/$uuid the container is exited and is garbage
	isExitedDeleting bool // when locked at pods/exited-garbage/$uuid the container is exited, garbage, and is being actively deleted
	isGarbage        bool // when unlocked at pods/garbage/$uuid the container is garbage that never ran
	isDeleting       bool // when locked at pods/garbage/$uuid the container is garbage that never ran, and is being actively deleted
	isGone           bool // when a container no longer can be located at its uuid anywhere XXX: only set by refreshState()
}

type includeMask byte

const (
	includeEmbryoDir includeMask = 1 << iota
	includePrepareDir
	includePreparedDir
	includeRunDir
	includeExitedGarbageDir
	includeGarbageDir

	includeMostDirs includeMask = (includeRunDir | includeExitedGarbageDir | includePrepareDir | includePreparedDir)
	includeAllDirs  includeMask = (includeMostDirs | includeEmbryoDir | includeGarbageDir)
)

var (
	containersInitialized = false
)

// initContainers creates the required global directories
func initContainers() error {
	if !containersInitialized {
		dirs := []string{embryoDir(), prepareDir(), preparedDir(), runDir(), exitedGarbageDir(), garbageDir()}
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
			c.isExitedGarbage && include&includeExitedGarbageDir == 0 ||
			c.isGarbage && include&includeGarbageDir == 0 ||
			c.isPrepared && include&includePreparedDir == 0 ||
			((c.isPreparing || c.isAbortedPrepare) && include&includePrepareDir == 0) ||
			c.isRunning() && include&includeRunDir == 0 {
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
		// rest start false.
	}

	var err error
	c.uuid, err = types.NewUUID(uuid.New())
	if err != nil {
		return nil, fmt.Errorf("error creating UUID: %v", err)
	}

	err = os.Mkdir(c.embryoPath(), 0700)
	if err != nil {
		return nil, err
	}

	c.FileLock, err = lock.NewLock(c.embryoPath(), lock.Dir)
	if err != nil {
		os.Remove(c.embryoPath())
		return nil, err
	}

	err = c.xToPreparing()
	if err != nil {
		return nil, err
	}

	// At this point we we have:
	// /var/lib/rkt/pods/prepare/$uuid << exclusively locked to indicate "preparing"

	return c, nil
}

// getContainer returns a container struct representing the given container.
// The returned lock is always left in an open but unlocked state.
// The container must be closed using container.Close()
func getContainer(uuid string) (*container, error) {
	if err := initContainers(); err != nil {
		return nil, err
	}

	c := &container{}

	u, err := types.NewUUID(uuid)
	if err != nil {
		return nil, err
	}
	c.uuid = u

	// we try open the container in all possible directories, in the same order the states occur
	l, err := lock.NewLock(c.embryoPath(), lock.Dir)
	if err == nil {
		c.isEmbryo = true
	} else if err == lock.ErrNotExist {
		l, err = lock.NewLock(c.preparePath(), lock.Dir)
		if err == nil {
			// treat as aborted prepare until lock is tested
			c.isAbortedPrepare = true
		} else if err == lock.ErrNotExist {
			l, err = lock.NewLock(c.preparedPath(), lock.Dir)
			if err == nil {
				c.isPrepared = true
			} else if err == lock.ErrNotExist {
				l, err = lock.NewLock(c.runPath(), lock.Dir)
				if err == nil {
					// treat as exited until lock is tested
					c.isExited = true
				} else if err == lock.ErrNotExist {
					l, err = lock.NewLock(c.exitedGarbagePath(), lock.Dir)
					if err == lock.ErrNotExist {
						l, err = lock.NewLock(c.garbagePath(), lock.Dir)
						if err == nil {
							c.isGarbage = true
						} else {
							return nil, fmt.Errorf("container %q not found", uuid)
						}
					} else if err == nil {
						c.isExitedGarbage = true
						c.isExited = true // ExitedGarbage is _always_ implicitly exited
					}
				}
			}
		}
	}

	if err != nil && err != lock.ErrNotExist {
		return nil, fmt.Errorf("error opening container %q: %v", uuid, err)
	}

	if !c.isPrepared && !c.isEmbryo {
		// preparing, run, exitedGarbage, and garbage dirs use exclusive locks to indicate preparing/aborted, running/exited, and deleting/marked
		if err = l.TrySharedLock(); err != nil {
			if err != lock.ErrLocked {
				l.Close()
				return nil, fmt.Errorf("unexpected lock error: %v", err)
			}
			if c.isExitedGarbage {
				// locked exitedGarbage is also being deleted
				c.isExitedDeleting = true
			} else if c.isExited {
				// locked exited and !exitedGarbage is not exited (default in the run dir)
				c.isExited = false
			} else if c.isAbortedPrepare {
				// locked in preparing is preparing, not aborted (default in the preparing dir)
				c.isAbortedPrepare = false
				c.isPreparing = true
			} else if c.isGarbage {
				// locked in non-exited garbage is deleting
				c.isDeleting = true
			}
			err = nil
		} else {
			l.Unlock()
		}
	}

	c.FileLock = l

	if c.isRunning() {
		cfd, err := c.Fd()
		if err != nil {
			return nil, fmt.Errorf("error acquiring container %v dir fd: %v", uuid, err)
		}
		c.nets, err = netinfo.LoadAt(cfd)
		// ENOENT is ok -- assume running without --private-net
		if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("error opening container %v netinfo: %v", uuid, err)
		}
	}

	return c, nil
}

// path returns the path to the container according to the current (cached) state.
func (c *container) path() string {
	if c.isEmbryo {
		return c.embryoPath()
	} else if c.isPreparing || c.isAbortedPrepare {
		return c.preparePath()
	} else if c.isPrepared {
		return c.preparedPath()
	} else if c.isExitedGarbage {
		return c.exitedGarbagePath()
	} else if c.isGarbage {
		return c.garbagePath()
	} else if c.isGone {
		return "" // TODO(vc): anything better?
	}

	return c.runPath()
}

// embryoPath returns the path to the container where it would be in the embryoDir in its embryonic state.
func (c *container) embryoPath() string {
	return filepath.Join(embryoDir(), c.uuid.String())
}

// preparePath returns the path to the container where it would be in the prepareDir in its preparing state.
func (c *container) preparePath() string {
	return filepath.Join(prepareDir(), c.uuid.String())
}

// preparedPath returns the path to the container where it would be in the preparedDir.
func (c *container) preparedPath() string {
	return filepath.Join(preparedDir(), c.uuid.String())
}

// runPath returns the path to the container where it would be in the runDir.
func (c *container) runPath() string {
	return filepath.Join(runDir(), c.uuid.String())
}

// exitedGarbagePath returns the path to the container where it would be in the exitedGarbageDir.
func (c *container) exitedGarbagePath() string {
	return filepath.Join(exitedGarbageDir(), c.uuid.String())
}

// garbagePath returns the path to the container where it would be in the garbageDir.
func (c *container) garbagePath() string {
	return filepath.Join(garbageDir(), c.uuid.String())
}

// xToPrepare transitions a container from embryo -> preparing, leaves the container locked in the prepare directory.
// only the creator of the container (via newContainer()) may do this, nobody to race with.
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

	if err := os.Rename(c.embryoPath(), c.preparePath()); err != nil {
		return err
	}

	c.isEmbryo = false
	c.isPreparing = true

	return nil
}

// xToPrepared transitions a container from preparing -> prepared, leaves the container unlocked in the prepared directory.
// only the creator of the container (via newContainer()) may do this, nobody to race with.
func (c *container) xToPrepared() error {
	if !c.createdByMe {
		return fmt.Errorf("bug: only containers created by me may transition to prepared")
	}

	if !c.isPreparing {
		return fmt.Errorf("bug: only preparing containers may transition to prepared")
	}

	if err := os.Rename(c.path(), c.preparedPath()); err != nil {
		return err
	}

	if err := c.Unlock(); err != nil {
		return err
	}

	c.isPreparing = false
	c.isPrepared = true

	return nil
}

// xToRun transitions a container from prepared -> run, leaves the container locked in the run directory.
// the creator of the container (via newContainer()) may also jump directly from preparing -> run
func (c *container) xToRun() error {
	if !c.createdByMe && !c.isPrepared {
		return fmt.Errorf("bug: only prepared containers may transition to run")
	}

	if c.createdByMe && !c.isPrepared && !c.isPreparing {
		return fmt.Errorf("bug: only prepared or preparing containers may transition to run")
	}

	if err := c.ExclusiveLock(); err != nil {
		return err
	}

	if err := os.Rename(c.path(), c.runPath()); err != nil {
		// TODO(vc): we could race here with a concurrent xToRun(), let caller deal with the error.
		return err
	}

	c.isPreparing = false
	c.isPrepared = false

	return nil
}

// xToExitedGarbage transitions a container from run -> exitedGarbage
func (c *container) xToExitedGarbage() error {
	if !c.isExited || c.isExitedGarbage {
		return fmt.Errorf("bug: only exited non-garbage containers may transition to exited-garbage")
	}

	if err := os.Rename(c.runPath(), c.exitedGarbagePath()); err != nil {
		// TODO(vc): another case where we could race with a concurrent xToExitedGarbage(), let caller deal with the error.
		return err
	}

	c.isExitedGarbage = true

	return nil
}

// xToGarbage transitions a container from prepared -> garbage or prepared -> garbage
func (c *container) xToGarbage() error {
	if !c.isAbortedPrepare && !c.isPrepared {
		return fmt.Errorf("bug: only failed prepare or prepared containers may transition to garbage")
	}

	if err := os.Rename(c.path(), c.garbagePath()); err != nil {
		return err
	}

	c.isAbortedPrepare = false
	c.isPrepared = false
	c.isGarbage = true

	return nil
}

// isRunning does the annoying tests to infer if a container is in a running state
func (c *container) isRunning() bool {
	// when none of these things, running!
	return !c.isEmbryo && !c.isAbortedPrepare && !c.isPreparing && !c.isPrepared && !c.isExited && !c.isExitedGarbage && !c.isGarbage && !c.isGone
}

// listContainers returns a list of container uuids in string form.
func listContainers(include includeMask) ([]string, error) {
	// uniqued due to the possibility of a container being renamed from across directories during the list operation
	ucs := make(map[string]struct{})
	dirs := []struct {
		kind includeMask
		path string
	}{
		{ // the order here is significant: embryo -> preparing -> prepared -> running -> exitedGarbage
			kind: includeEmbryoDir,
			path: embryoDir(),
		}, {
			kind: includePrepareDir,
			path: prepareDir(),
		}, {
			kind: includePreparedDir,
			path: preparedDir(),
		}, {
			kind: includeRunDir,
			path: runDir(),
		}, {
			kind: includeExitedGarbageDir,
			path: exitedGarbageDir(),
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
// assumes c.FileLock is currently unlocked, and always returns with it unlocked.
func (c *container) refreshState() error {
	//  TODO(vc): this overlaps substantially with newContainer(), could probably unify.
	c.isEmbryo = false
	c.isPreparing = false
	c.isAbortedPrepare = false
	c.isPrepared = false
	c.isExited = false
	c.isExitedGarbage = false
	c.isExitedDeleting = false
	c.isGarbage = false
	c.isDeleting = false
	c.isGone = false

	// we try open the container in all possible directories, in the same order the states occur
	_, err := os.Stat(c.embryoPath())
	if err == nil {
		c.isEmbryo = true
	} else if os.IsNotExist(err) {
		_, err := os.Stat(c.preparePath())
		if err == nil {
			// treat as aborted prepare until lock is tested
			c.isAbortedPrepare = true
		} else if os.IsNotExist(err) {
			_, err := os.Stat(c.preparedPath())
			if err == nil {
				c.isPrepared = true
			} else if os.IsNotExist(err) {
				_, err := os.Stat(c.runPath())
				if err == nil {
					// treat as exited until lock is tested
					c.isExited = true
				} else if os.IsNotExist(err) {
					_, err := os.Stat(c.exitedGarbagePath())
					if os.IsNotExist(err) {
						_, err := os.Stat(c.garbagePath())
						if os.IsNotExist(err) {
							// XXX: note this is unique to refreshState(), getContainer() errors when it can't find a uuid.
							c.isGone = true
						} else if err == nil {
							c.isGarbage = true
						}
					} else if err == nil {
						c.isExitedGarbage = true
						c.isExited = true // exitedGarbage is _always_ implicitly exited
					}
				}
			}
		}
	}

	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("error refreshing state of container %q: %v", c.uuid.String(), err)
	}

	if !c.isPrepared && !c.isEmbryo && !c.isGone {
		// preparing, run, and exitedGarbage dirs use exclusive locks to indicate preparing/aborted, running/exited, and deleting/marked
		if err = c.TrySharedLock(); err != nil {
			if err != lock.ErrLocked {
				c.Close()
				return fmt.Errorf("unexpected lock error: %v", err)
			}
			if c.isExitedGarbage {
				// locked exitedGarbage is also being deleted
				c.isExitedDeleting = true
			} else if c.isExited {
				// locked exited and !exitedGarbage is not exited (default in the run dir)
				c.isExited = false
			} else if c.isAbortedPrepare {
				// locked in preparing is preparing, not aborted (default in the preparing dir)
				c.isAbortedPrepare = false
				c.isPreparing = true
			} else if c.isGarbage {
				// locked in non-exited garbage is deleting
				c.isDeleting = true
			}
			err = nil
		} else {
			c.Unlock()
		}
	}

	return nil
}

// waitExited waits for a container to (run and) exit.
func (c *container) waitExited() error {
	for !c.isExited && !c.isAbortedPrepare && !c.isGarbage && !c.isGone {
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

	// TODO(vc): return error or let caller detect the !c.isExited possibilities?

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
	} else if c.isAbortedPrepare {
		state = "aborted prepare"
	} else if c.isPrepared {
		state = "prepared"
	} else if c.isExitedDeleting || c.isDeleting {
		state = "deleting"
	} else if c.isExited { // this covers c.isExitedGarbage
		state = "exited"
	} else if c.isGarbage {
		state = "garbage"
	}

	return state
}

// getPID returns the pid of the container.
func (c *container) getPID() (int, error) {
	return c.readIntFromFile("pid")
}

// getStage1Hash returns the hash of the stage1 image used in this container
func (c *container) getStage1Hash() (*types.Hash, error) {
	s1IDb, err := c.readFile(common.Stage1IDFilename)
	if err != nil {
		return nil, err
	}
	s1img, err := types.NewHash(string(s1IDb))
	if err != nil {
		return nil, err
	}

	return s1img, nil
}

// getAppsHashes returns a list of the app hashes in the container
func (c *container) getAppsHashes() ([]types.Hash, error) {
	pmb, err := c.readFile("pod")
	if err != nil {
		return nil, err
	}
	var pm *schema.PodManifest
	if err = json.Unmarshal(pmb, &pm); err != nil {
		return nil, err
	}

	var imgs []types.Hash
	for _, app := range pm.Apps {
		imgs = append(imgs, app.Image.ID)
	}

	return imgs, nil
}

// getDirNames returns the list of names from a container's directory
func (c *container) getDirNames(path string) ([]string, error) {
	dir, err := c.openFile(path, syscall.O_RDONLY|syscall.O_DIRECTORY)
	if err != nil {
		return nil, fmt.Errorf("unable to open directory: %v", err)
	}
	defer dir.Close()

	ld, err := dir.Readdirnames(0)
	if err != nil {
		return nil, fmt.Errorf("unable to read directory: %v", err)
	}

	return ld, nil
}

// getAppCount returns the app count of a container. It can only be called on prepared containers.
func (c *container) getAppCount() (int, error) {
	if !c.isPrepared {
		return -1, fmt.Errorf("error: only prepared containers can get their app count")
	}

	b, err := ioutil.ReadFile(common.PodManifestPath(c.path()))
	if err != nil {
		return -1, fmt.Errorf("error reading container manifest: %v", err)
	}

	m := schema.PodManifest{}
	if err = m.UnmarshalJSON(b); err != nil {
		return -1, fmt.Errorf("unable to load manifest: %v", err)
	}

	return len(m.Apps), nil
}

func (c *container) getStatusDir() (string, error) {
	_, err := c.openFile(common.OverlayPreparedFilename, syscall.O_RDONLY)
	if err == nil {
		// the container uses overlay. Since the mount is in another mount
		// namespace (or gone), return the status directory from the overlay
		// upper layer
		stage1Hash, err := c.getStage1Hash()
		if err != nil {
			return "", err
		}
		overlayStatusDir := fmt.Sprintf(overlayStatusDirTemplate, stage1Hash.String())

		return overlayStatusDir, nil
	}

	// not using overlay, return the regular status directory
	return regularStatusDir, nil
}

// getExitStatuses returns a map of the statuses of the container.
func (c *container) getExitStatuses() (map[string]int, error) {
	statusDir, err := c.getStatusDir()
	if err != nil {
		return nil, fmt.Errorf("unable to get status directory: %v", err)
	}
	ls, err := c.getDirNames(statusDir)
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

//stop stops the container
func (c *container) stop() (err error) {
	if c.isExited {
		return fmt.Errorf("already stopped")
	}

	pid, err := c.getPID()
	if err != nil {
		return err
	}

	err = syscall.Kill(pid, syscall.SIGSTOP)
	if err != nil {
		return err
	}

	return nil
}
