// Copyright 2014 The rkt Authors
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
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/spf13/cobra"
	"github.com/coreos/rkt/common"
	"github.com/coreos/rkt/stage0"
	"github.com/coreos/rkt/store"
)

const (
	defaultGracePeriod        = 30 * time.Minute
	defaultPreparedExpiration = 24 * time.Hour
)

var (
	cmdGC = &cobra.Command{
		Use:   "gc [--grace-period=duration] [--expire-prepared=duration]",
		Short: "Garbage-collect rkt pods no longer in use",
		Run:   runWrapper(runGC),
	}
	flagGracePeriod        time.Duration
	flagPreparedExpiration time.Duration
)

func init() {
	cmdRkt.AddCommand(cmdGC)
	cmdGC.Flags().DurationVar(&flagGracePeriod, "grace-period", defaultGracePeriod, "duration to wait before discarding inactive pods from garbage")
	cmdGC.Flags().DurationVar(&flagPreparedExpiration, "expire-prepared", defaultPreparedExpiration, "duration to wait before expiring prepared pods")
}

func runGC(cmd *cobra.Command, args []string) (exit int) {
	if err := renameExited(); err != nil {
		stderr("Failed to rename exited pods: %v", err)
		return 1
	}

	if err := renameAborted(); err != nil {
		stderr("Failed to rename aborted pods: %v", err)
		return 1
	}

	if err := renameExpired(flagPreparedExpiration); err != nil {
		stderr("Failed to rename expired prepared pods: %v", err)
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

// renameExited renames exited pods to the exitedGarbage directory
func renameExited() error {
	if err := walkPods(includeRunDir, func(p *pod) {
		if p.isExited {
			stderr("Moving pod %q to garbage", p.uuid)
			if err := p.xToExitedGarbage(); err != nil && err != os.ErrNotExist {
				stderr("Rename error: %v", err)
			}
		}
	}); err != nil {
		return err
	}

	return nil
}

// emptyExitedGarbage discards sufficiently aged pods from exitedGarbageDir()
func emptyExitedGarbage(gracePeriod time.Duration) error {
	if err := walkPods(includeExitedGarbageDir, func(p *pod) {
		gp := p.path()
		st := &syscall.Stat_t{}
		if err := syscall.Lstat(gp, st); err != nil {
			if err != syscall.ENOENT {
				stderr("Unable to stat %q, ignoring: %v", gp, err)
			}
			return
		}

		if expiration := time.Unix(st.Ctim.Unix()).Add(gracePeriod); time.Now().After(expiration) {
			if err := p.ExclusiveLock(); err != nil {
				return
			}
			stdout("Garbage collecting pod %q", p.uuid)

			deletePod(p)
		}
	}); err != nil {
		return err
	}

	return nil
}

// renameAborted renames failed prepares to the garbage directory
func renameAborted() error {
	if err := walkPods(includePrepareDir, func(p *pod) {
		if p.isAbortedPrepare {
			stderr("Moving failed prepare %q to garbage", p.uuid)
			if err := p.xToGarbage(); err != nil && err != os.ErrNotExist {
				stderr("Rename error: %v", err)
			}
		}
	}); err != nil {
		return err
	}
	return nil
}

// renameExpired renames expired prepared pods to the garbage directory
func renameExpired(preparedExpiration time.Duration) error {
	if err := walkPods(includePreparedDir, func(p *pod) {
		st := &syscall.Stat_t{}
		pp := p.path()
		if err := syscall.Lstat(pp, st); err != nil {
			if err != syscall.ENOENT {
				stderr("Unable to stat %q, ignoring: %v", pp, err)
			}
			return
		}

		if expiration := time.Unix(st.Ctim.Unix()).Add(preparedExpiration); time.Now().After(expiration) {
			stderr("Moving expired prepared pod %q to garbage", p.uuid)
			if err := p.xToGarbage(); err != nil && err != os.ErrNotExist {
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
	if err := walkPods(includeGarbageDir, func(p *pod) {
		if err := p.ExclusiveLock(); err != nil {
			return
		}
		stdout("Garbage collecting pod %q", p.uuid)

		deletePod(p)
	}); err != nil {
		return err
	}

	return nil
}

// deletePod cleans up files and resource associated with the pod
// pod must be under exclusive lock and be in either ExitedGarbage
// or Garbage state
func deletePod(p *pod) {
	if !p.isExitedGarbage && !p.isGarbage {
		panic("logic error: deletePod called with non-garbage pod")
	}

	if p.isExitedGarbage {
		s, err := store.NewStore(globalFlags.Dir)
		if err != nil {
			stderr("Cannot open store: %v", err)
			return
		}
		stage1TreeStoreID, err := p.getStage1TreeStoreID()
		if err != nil {
			stderr("Error getting stage1 treeStoreID: %v", err)
			return
		}
		stage1RootFS := s.GetTreeStoreRootFS(stage1TreeStoreID)

		// execute stage1's GC
		if err := stage0.GC(p.path(), p.uuid, stage1RootFS, globalFlags.Debug); err != nil {
			stderr("Stage1 GC of pod %q failed: %v", p.uuid, err)
			return
		}

		if p.usesOverlay() {
			apps, err := p.getApps()
			if err != nil {
				stderr("Error retrieving app hashes from pod manifest: %v", err)
				return
			}
			for _, a := range apps {
				dest := filepath.Join(common.AppPath(p.path(), a.Name), "rootfs")
				if err := syscall.Unmount(dest, 0); err != nil {
					// machine could have been rebooted and mounts lost.
					// ignore "does not exist" and "not a mount point" errors
					if err != syscall.ENOENT && err != syscall.EINVAL {
						stderr("Error unmounting app at %v: %v", dest, err)
					}
				}
			}

			s1 := filepath.Join(common.Stage1ImagePath(p.path()), "rootfs")
			if err := syscall.Unmount(s1, 0); err != nil {
				// machine could have been rebooted and mounts lost.
				// ignore "does not exist" and "not a mount point" errors
				if err != syscall.ENOENT && err != syscall.EINVAL {
					stderr("Error unmounting stage1 at %v: %v", s1, err)
					return
				}
			}
		}
	}

	if err := os.RemoveAll(p.path()); err != nil {
		stderr("Unable to remove pod %q: %v", p.uuid, err)
	}
}
