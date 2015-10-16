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
	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/schema/types"
	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/spf13/cobra"
	"github.com/coreos/rkt/store"
)

var (
	cmdImageRm = &cobra.Command{
		Use:   "rm IMAGEID...",
		Short: "Remove image(s) with the given ID(s) from the local store",
		Run:   runWrapper(runRmImage),
	}
)

func init() {
	cmdImage.AddCommand(cmdImageRm)
}

func runRmImage(cmd *cobra.Command, args []string) (exit int) {
	if len(args) < 1 {
		stderr("rkt: Must provide at least one image ID")
		return 1
	}

	s, err := store.NewStore(globalFlags.Dir)
	if err != nil {
		stderr("rkt: cannot open store: %v", err)
		return 1
	}

	done := 0
	errors := 0
	staleErrors := 0
	for _, pkey := range args {
		errors++
		h, err := types.NewHash(pkey)
		if err != nil {
			stderr("rkt: wrong image ID %q: %v", pkey, err)
			continue
		}
		key, err := s.ResolveKey(h.String())
		if err != nil {
			stderr("rkt: image ID %q not valid: %v", pkey, err)
			continue
		}
		if key == "" {
			stderr("rkt: image ID %q doesn't exist", pkey)
			continue
		}

		if err = s.RemoveACI(key); err != nil {
			if serr, ok := err.(*store.StoreRemovalError); ok {
				staleErrors++
				stderr("rkt: some files cannot be removed for image ID %q: %v", pkey, serr)
			} else {
				stderr("rkt: error removing aci for image ID %q: %v", pkey, err)
				continue
			}
		}
		stdout("rkt: successfully removed aci for image ID: %q", pkey)
		errors--
		done++
	}

	if done > 0 {
		stderr("rkt: %d image(s) successfully removed", done)
	}

	// If anything didn't complete, return exit status of 1
	if (errors + staleErrors) > 0 {
		if staleErrors > 0 {
			stderr("rkt: %d image(s) removed but left some stale files", staleErrors)
		}
		if errors > 0 {
			stderr("rkt: %d image(s) cannot be removed", errors)
		}
		return 1
	}

	return 0
}
