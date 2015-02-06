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

package main

import (
	"os"
	"syscall"
	"time"
)

const (
	defaultGracePeriod = 30 * time.Minute
)

var (
	flagGracePeriod time.Duration
	cmdGC           = &Command{
		Name:    "gc",
		Summary: "Garbage-collect rkt containers no longer in use",
		Usage:   "[--grace-period=duration]",
		Run:     runGC,
	}
)

func init() {
	commands = append(commands, cmdGC)
	cmdGC.Flags.DurationVar(&flagGracePeriod, "grace-period", defaultGracePeriod, "duration to wait before discarding inactive containers from garbage")
}

func runGC(args []string) (exit int) {
	if err := os.MkdirAll(garbageDir(), 0755); err != nil {
		stderr("Unable to create garbage dir: %v", err)
		return 1
	}

	if err := renameExited(); err != nil {
		stderr("Failed to rename exited containers: %v", err)
		return 1
	}

	// clean up anything old in the garbage dir
	if err := emptyGarbage(flagGracePeriod); err != nil {
		stderr("Failed to empty garbage: %v", err)
		return 1
	}

	return
}

// renameExited renames exited containers to the garbage directory
func renameExited() error {
	if err := walkContainers(includeContainersDir, func(c *container) {
		if c.isExited {
			stdout("Moving container %q to garbage", c.uuid)
			if err := os.Rename(c.containersPath(), c.garbagePath()); err != nil && err != os.ErrNotExist {
				stderr("Rename error: %v", err)
			}
		}
	}); err != nil {
		return err
	}

	return nil
}

// emptyGarbage discards sufficiently aged containers from garbageDir()
func emptyGarbage(gracePeriod time.Duration) error {
	if err := walkContainers(includeGarbageDir, func(c *container) {
		gp := c.path()
		st := &syscall.Stat_t{}
		if err := syscall.Lstat(gp, st); err != nil {
			if err != syscall.ENOENT {
				stderr("Unable to stat %q, ignoring: %v", gp, err)
			}
			return
		}

		if expiration := time.Unix(st.Ctim.Unix()).Add(gracePeriod); time.Now().After(expiration) {
			if err := c.ExclusiveLock(); err != nil {
				return
			}
			stdout("Garbage collecting container %q", c.uuid)
			if err := os.RemoveAll(gp); err != nil {
				stderr("Unable to remove container %q: %v", c.uuid, err)
			}
		}
	}); err != nil {
		return err
	}

	return nil
}
