// Copyright 2014 CoreOS, Inc.
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
	"syscall"

	"github.com/appc/spec/schema/types"
	"github.com/coreos/rocket/pkg/lock"
)

var (
	cmdStatus = &Command{
		Name:    "status",
		Summary: "Check the status of a rkt job",
		Usage:   "--uuid UUID [--wait]",
		Run:     runStatus,
	}
	flagContainerUUID types.UUID
	flagWait          bool
)

func init() {
	commands = append(commands, cmdStatus)
	cmdStatus.Flags.Var(&flagContainerUUID, "uuid", "uuid of container to query the status of")
	cmdStatus.Flags.BoolVar(&flagWait, "wait", false, "toggle waiting for the container to exit if running")
}

func runStatus(args []string) (exit int) {
	if flagContainerUUID.Empty() {
		fmt.Fprintf(os.Stderr, "--uuid required\n")
		return 1
	}
	id := flagContainerUUID.String()

	isGarbage := false
	exited := true

	cp := filepath.Join(containersDir(), id)
	var l lock.DirLock
	var err error
	// First check if an active container exists
	if flagWait {
		// If necessary, block until the lock can be obtained
		l, err = lock.SharedLock(cp)
	} else {
		l, err = lock.TrySharedLock(cp)
	}
	if isNoSuchDirErr(err) {
		// Fall back to checking garbage directory
		cp = filepath.Join(garbageDir(), id)
		l, err = lock.TrySharedLock(cp)
		isGarbage = true
	}
	switch {
	case err == lock.ErrLocked:
		if isGarbage {
			fmt.Fprintf(os.Stderr, "Container locked by another process\n")
			return 1
		}
		exited = false
	case isNoSuchDirErr(err):
		fmt.Fprintf(os.Stderr, "No such container: %s\n", id)
		return 1
	case err != nil:
		fmt.Fprintf(os.Stderr, "Error locking container: %v\n", err)
		return 1
	default:
		defer l.Close()
	}

	if err := printStatus(cp, exited); err != nil {
		fmt.Fprintf(os.Stderr, "Unable to print status: %v\n", err)
		return 1
	}

	return 0
}

// printStatus prints the container's pid and per-app status codes
func printStatus(cpath string, exited bool) error {
	pid, err := getIntFromFile(filepath.Join(cpath, "pid"))
	if err != nil {
		return err
	}

	stats, err := getStatuses(cpath)
	if err != nil {
		return err
	}

	fmt.Printf("pid=%d\nexited=%t\n", pid, exited)
	for app, stat := range stats {
		fmt.Printf("%s=%d\n", app, stat)
	}
	return nil
}

// getStatuses returns a map of imageId:status codes for the given container
func getStatuses(cpath string) (map[string]int, error) {
	sdir := filepath.Join(cpath, "stage1/rkt/status")
	ls, err := ioutil.ReadDir(sdir)
	if err != nil {
		return nil, fmt.Errorf("unable to read status directory: %v", err)
	}

	stats := make(map[string]int)
	for _, ent := range ls {
		if ent.IsDir() {
			continue
		}
		s, err := getIntFromFile(filepath.Join(sdir, ent.Name()))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to get status of app %q: %v\n", ent.Name(), err)
			continue
		}
		stats[ent.Name()] = s
	}

	return stats, err
}

// getIntFromFile reads an integer string from the named file
func getIntFromFile(path string) (i int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	buf, err := ioutil.ReadAll(f)
	if err != nil {
		return
	}

	_, err = fmt.Sscanf(string(buf), "%d", &i)
	if err != nil {
		return
	}

	return
}

func isNoSuchDirErr(err error) bool {
	e, ok := err.(syscall.Errno)
	return ok && e == syscall.ENOENT
}
