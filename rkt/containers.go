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

	"github.com/coreos/rocket/pkg/lock"
)

type container struct {
	*lock.DirLock
	uuid       string
	isExited   bool
	isGarbage  bool
	isDeleting bool
	createdAt  time.Time
	garbageAt  time.Time
}

type includeMask byte

const (
	includeContainersDir includeMask = 1 << iota
	includeGarbageDir
)

// walkContainers iterates over the included containers calling function f for every container.
func walkContainers(include includeMask, f func(*container)) error {
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

		// omit garbage containers if garbage wasn't included
		// this is to cover a race between listContainers finding the uuids and `rkt gc` renaming a container
		if c.isGarbage && include&includeGarbageDir == 0 {
			c.Close()
			continue
		}

		f(c)
		c.Close()
	}
	return nil
}

// getContainer returns a container struct representing the given container.
// The returned lock is always left in an open but unlocked state.
// The container must be closed using container.Close()
func getContainer(uuid string) (*container, error) {
	c := &container{
		uuid:       uuid,
		isGarbage:  true,
		isExited:   true,
		isDeleting: false,
	}

	l, err := lock.NewLock(filepath.Join(garbageDir(), uuid))
	if err == lock.ErrNotExist {
		l, err = lock.NewLock(filepath.Join(containersDir(), uuid))
		c.isGarbage = false
		c.isExited = false
	}

	if err != nil {
		return nil, fmt.Errorf("error opening container %q: %v", uuid, err)
	}

	if err = l.TrySharedLock(); err != nil {
		if err != lock.ErrLocked {
			l.Close()
			return nil, fmt.Errorf("unexpected lock error: %v", err)
		}
		if c.isGarbage {
			c.isDeleting = true
		}
	} else {
		l.Unlock()
		c.isExited = true
	}

	c.DirLock = l
	cfd, err := c.Fd()
	if err != nil {
		c.Close()
		return nil, fmt.Errorf("error getting lock fd: %v", err)
	}

	var st syscall.Stat_t
	if err := syscall.Fstat(cfd, &st); err != nil {
		return nil, fmt.Errorf("error stating container: %v", err)
	}

	// The container directory's mtime is approximately the time it was created
	c.createdAt = time.Unix(st.Mtim.Unix())

	// If the container is garbage, its Ctim reflects when it was marked as such
	if c.isGarbage {
		c.garbageAt = time.Unix(st.Ctim.Unix())
	}

	return c, nil
}

// path returns the path to the container where it was opened.
func (c *container) path() string {
	if c.isGarbage {
		return c.garbagePath()
	}
	return c.containersPath()
}

// garbagePath returns the path to the container where it would be in the garbageDir.
func (c *container) garbagePath() string {
	return filepath.Join(garbageDir(), c.uuid)
}

// containersPath returns the path to the container where it would be in the containersDir.
func (c *container) containersPath() string {
	return filepath.Join(containersDir(), c.uuid)
}

// listContainers returns a list of container uuids in string form.
func listContainers(include includeMask) ([]string, error) {
	// uniqued due to the possibility of a container being renamed from containersDir to garbageDir during this operation
	ucs := make(map[string]struct{})

	if include&includeContainersDir != 0 {
		cs, err := listContainersFromDir(containersDir())
		if err != nil {
			return nil, err
		}

		for _, c := range cs {
			ucs[c] = struct{}{}
		}
	}

	if include&includeGarbageDir != 0 {
		cs, err := listContainersFromDir(garbageDir())
		if err != nil {
			return nil, err
		}

		for _, c := range cs {
			ucs[c] = struct{}{}
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

// waitExited waits for a container to exit.
func (c *container) waitExited() error {
	if !c.isExited {
		if err := c.SharedLock(); err != nil {
			return err
		}
		c.isExited = true
	}
	return nil
}

// readFile reads an entire file from a container's directory
func (c *container) readFile(path string) ([]byte, error) {
	f, err := c.openFile(path, syscall.O_RDONLY)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return ioutil.ReadAll(f)
}

// readIntFromFile reads an int from a file in a container's directory
func (c *container) readIntFromFile(path string) (i int, err error) {
	b, err := c.readFile(path)
	if err != nil {
		return
	}
	_, err = fmt.Sscanf(string(b), "%d", &i)
	return
}

// openFile opens a file from a container's directory returning a file descriptor
func (c *container) openFile(path string, flags int) (*os.File, error) {
	cdirfd, err := c.Fd()
	if err != nil {
		return nil, err
	}

	fd, err := syscall.Openat(cdirfd, path, flags, 0)
	if err != nil {
		return nil, fmt.Errorf("unable to open status directory: %v", err)
	}

	return os.NewFile(uintptr(fd), path), nil
}

// getPID returns the pid of the container
func (c *container) getPID() (int, error) {
	return c.readIntFromFile("pid")
}

// getStatuses returns a map of the statuses of the container
func (c *container) getStatuses() (map[string]int, error) {
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

