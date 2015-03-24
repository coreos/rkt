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
	"flag"
	"os"
	"syscall"
	"time"
)

const (
	defaultGracePeriod        = 30 * time.Minute
	defaultPreparedExpiration = 24 * time.Hour
)

var (
	cmdGC = &Command{
		Name:    "gc",
		Summary: "Garbage-collect rkt containers no longer in use",
		Usage:   "[--grace-period=duration] [--expire-prepared=duration]",
		Run:     runGC,
		Flags:   &gcFlags,
	}
	gcFlags                flag.FlagSet
	flagGracePeriod        time.Duration
	flagPreparedExpiration time.Duration
)

func init() {
	commands = append(commands, cmdGC)
	gcFlags.DurationVar(&flagGracePeriod, "grace-period", defaultGracePeriod, "duration to wait before discarding inactive containers from garbage")
	gcFlags.DurationVar(&flagPreparedExpiration, "expire-prepared", defaultPreparedExpiration, "duration to wait before expiring prepared containers")
}

func runGC(args []string) (exit int) {
	if err := renameExited(); err != nil {
		stderr("Failed to rename exited containers: %v", err)
		return 1
	}

	if err := renameAborted(); err != nil {
		stderr("Failed to rename aborted containers: %v", err)
		return 1
	}

	if err := renameExpired(flagPreparedExpiration); err != nil {
		stderr("Failed to rename expired prepared containers: %v", err)
		return 1
	}

	if err := emptyExitedGarbage(flagGracePeriod); err != nil {
		stderr("Failed to empty exitedGarbage: %v", err)
		return 1
	}

	if err := emptyGarbage(); err != nil {
		stderr("Failed to empty garbage: %v", err)
		return 1
	}

	return
}

// renameExited renames exited containers to the exitedGarbage directory
func renameExited() error {
	if err := walkContainers(includeRunDir, func(c *container) {
		if c.isExited {
			stdout("Moving container %q to garbage", c.uuid)
			if err := c.xToExitedGarbage(); err != nil && err != os.ErrNotExist {
				stderr("Rename error: %v", err)
			}
		}
	}); err != nil {
		return err
	}

	return nil
}

// emptyExitedGarbage discards sufficiently aged containers from exitedGarbageDir()
func emptyExitedGarbage(gracePeriod time.Duration) error {
	if err := walkContainers(includeExitedGarbageDir, func(c *container) {
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

// renameAborted renames failed prepares to the garbage directory
func renameAborted() error {
	if err := walkContainers(includePrepareDir, func(c *container) {
		if c.isAbortedPrepare {
			stdout("Moving failed prepare %q to garbage", c.uuid)
			if err := c.xToGarbage(); err != nil && err != os.ErrNotExist {
				stderr("Rename error: %v", err)
			}
		}
	}); err != nil {
		return err
	}
	return nil
}

// renameExpired renames expired prepared containers to the garbage directory
func renameExpired(preparedExpiration time.Duration) error {
	if err := walkContainers(includePreparedDir, func(c *container) {
		st := &syscall.Stat_t{}
		cp := c.path()
		if err := syscall.Lstat(cp, st); err != nil {
			if err != syscall.ENOENT {
				stderr("Unable to stat %q, ignoring: %v", cp, err)
			}
			return
		}

		if expiration := time.Unix(st.Ctim.Unix()).Add(preparedExpiration); time.Now().After(expiration) {
			stdout("Moving expired prepared container %q to garbage", c.uuid)
			if err := c.xToGarbage(); err != nil && err != os.ErrNotExist {
				stderr("Rename error: %v", err)
			}
		}
	}); err != nil {
		return err
	}
	return nil
}

// emptyGarbage discards everything from garbageDir()
func emptyGarbage() error {
	if err := walkContainers(includeGarbageDir, func(c *container) {
		if err := c.ExclusiveLock(); err != nil {
			return
		}
		stdout("Garbage collecting container %q", c.uuid)
		if err := os.RemoveAll(c.path()); err != nil {
			stderr("Unable to remove container %q: %v", c.uuid, err)
		}
	}); err != nil {
		return err
	}

	return nil
}
