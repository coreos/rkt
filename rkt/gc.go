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
	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/spf13/cobra"
	"os"
	"syscall"
	"time"

	"github.com/coreos/rkt/stage0"
	"github.com/coreos/rkt/store"
)

const (
	defaultGracePeriod        = 30 * time.Minute
	defaultPreparedExpiration = 24 * time.Hour
)

var (
	flagGracePeriod        time.Duration
	flagPreparedExpiration time.Duration

	cmdGC = &cobra.Command{
		Use:   "gc [--grace-period=duration] [--expire-prepared=duration]",
		Short: "Garbage-collect rkt pods no longer in use",
		Run: func(cmd *cobra.Command, args []string) {
			subCmdExitCode = runGC(cmd, args)
		},
	}
)

func init() {
	cmdGC.Flags().DurationVarP(&flagGracePeriod, "grace-period", "g", defaultGracePeriod, "duration to wait before discarding inactive pods from garbage")
	cmdGC.Flags().DurationVarP(&flagPreparedExpiration, "expire-prepared", "e", defaultPreparedExpiration, "duration to wait before expiring prepared pods")
	rktCmd.AddCommand(cmdGC)
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
			stdout("Moving pod %q to garbage", p.uuid)
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

			s, err := store.NewStore(flagDataDir)
			if err != nil {
				stderr("Cannot open store: %v", err)
				return
			}
			stage1ID, err := p.getStage1Hash()
			if err != nil {
				stderr("Error getting stage1 hash")
				return
			}
			stage1RootFS := s.GetTreeStoreRootFS(stage1ID.String())

			// execute stage1's GC
			if err := stage0.GC(p.path(), p.uuid, stage1RootFS, flagDebug); err != nil {
				stderr("Stage1 GC of pod %q failed: %v", p.uuid, err)
				return
			}

			if err := os.RemoveAll(gp); err != nil {
				stderr("Unable to remove pod %q: %v", p.uuid, err)
			}
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
			stdout("Moving failed prepare %q to garbage", p.uuid)
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
			stdout("Moving expired prepared pod %q to garbage", p.uuid)
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
		if err := os.RemoveAll(p.path()); err != nil {
			stderr("Unable to remove pod %q: %v", p.uuid, err)
		}
	}); err != nil {
		return err
	}

	return nil
}
