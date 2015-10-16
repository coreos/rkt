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

package main

import (
	"fmt"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/spf13/cobra"
	"github.com/coreos/rkt/common"
	"github.com/coreos/rkt/pkg/lock"
	"github.com/coreos/rkt/store"
)

var (
	cmdImageGc = &cobra.Command{
		Use:   "gc",
		Short: "Garbage collect local store",
		Run:   runWrapper(runGcImage),
	}
)

func init() {
	cmdImage.AddCommand(cmdImageGc)
}

func runGcImage(cmd *cobra.Command, args []string) (exit int) {
	s, err := store.NewStore(globalFlags.Dir)
	if err != nil {
		stderr("rkt: cannot open store: %v", err)
		return 1
	}

	if err := gcTreeStore(s); err != nil {
		stderr("rkt: failed to remove unreferenced treestores: %v", err)
		return 1
	}
	return 0
}

// gcTreeStore removes all treeStoreIDs not referenced by any non garbage
// collected pod from the store.
func gcTreeStore(s *store.Store) error {
	// Take an exclusive lock to block other pods being created.
	// This is needed to avoid races between the below steps (getting the
	// list of referenced treeStoreIDs, getting the list of treeStoreIDs
	// from the store, removal of unreferenced treeStoreIDs) and new
	// pods/treeStores being created/referenced
	keyLock, err := lock.ExclusiveKeyLock(lockDir(), common.PrepareLock)
	if err != nil {
		return fmt.Errorf("cannot get exclusive prepare lock: %v", err)
	}
	defer keyLock.Close()
	referencedTreeStoreIDs, err := getReferencedTreeStoreIDs()
	if err != nil {
		return fmt.Errorf("cannot get referenced treestoreIDs: %v", err)
	}
	treeStoreIDs, err := s.GetTreeStoreIDs()
	if err != nil {
		return fmt.Errorf("cannot get treestoreIDs from the store: %v", err)
	}
	for _, treeStoreID := range treeStoreIDs {
		if _, ok := referencedTreeStoreIDs[treeStoreID]; !ok {
			if err := s.RemoveTreeStore(treeStoreID); err != nil {
				stderr("rkt: error removing treestore %q: %v", treeStoreID, err)
			} else {
				stderr("rkt: removed treestore %q", treeStoreID)
			}
		}
	}
	return nil
}

func getReferencedTreeStoreIDs() (map[string]struct{}, error) {
	treeStoreIDs := map[string]struct{}{}
	var walkErr error
	// Consider pods in preparing, prepared, run, exitedgarbage state
	if err := walkPods(includeMostDirs, func(p *pod) {
		stage1TreeStoreID, err := p.getStage1TreeStoreID()
		if err != nil {
			walkErr = fmt.Errorf("cannot get stage1 treestoreID for pod %s: %v", p.uuid, err)
			return
		}
		appsTreeStoreIDs, err := p.getAppsTreeStoreIDs()
		if err != nil {
			walkErr = fmt.Errorf("cannot get apps treestoreIDs for pod %s: %v", p.uuid, err)
			return
		}
		allTreeStoreIDs := append(appsTreeStoreIDs, stage1TreeStoreID)

		for _, treeStoreID := range allTreeStoreIDs {
			treeStoreIDs[treeStoreID] = struct{}{}
		}
	}); err != nil {
		return nil, fmt.Errorf("failed to get pod handles: %v", err)
	}
	if walkErr != nil {
		return nil, walkErr
	}
	return treeStoreIDs, nil
}
