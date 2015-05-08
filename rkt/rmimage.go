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
	"flag"
	"fmt"

	"github.com/coreos/rkt/store"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/schema/types"
)

var (
	cmdRmImage = &Command{
		Name:    "rmimage",
		Summary: "Remove image(s) with the given key(s) from the local store",
		Usage:   "IMAGEID...",
		Run:     runRmImage,
		Flags:   &rmImageFlags,
	}
	rmImageFlags flag.FlagSet
)

func init() {
	commands = append(commands, cmdRmImage)
}

func runRmImage(args []string) (exit int) {
	if len(args) < 1 {
		stderr("rkt: Must provide at least one image key")
		return 1
	}

	s, err := store.NewStore(globalFlags.Dir)
	if err != nil {
		stderr("rkt: cannot open store: %v\n", err)
		return 1
	}

	referencedImgs, err := getReferencedImgs(s)
	if err != nil {
		stderr("rkt: cannot get referenced images: %v\n", err)
		return 1
	}

	//TODO(sgotti) Which return code to use when the removal fails only for some images?
	done := 0
	errors := 0
	staleErrors := 0
	for _, pkey := range args {
		errors++
		h, err := types.NewHash(pkey)
		if err != nil {
			stderr("rkt: wrong imageID %q: %v\n", pkey, err)
			continue
		}
		key, err := s.ResolveKey(h.String())
		if err != nil {
			stderr("rkt: imageID %q not valid: %v\n", pkey, err)
			continue
		}
		if key == "" {
			stderr("rkt: imageID %q doesn't exists\n", pkey)
			continue
		}

		err = s.RemoveACI(key)
		if err != nil {
			if serr, ok := err.(*store.StoreRemovalError); ok {
				staleErrors++
				stderr("rkt: some files cannot be removed for imageID %q: %v\n", pkey, serr)
			} else {
				stderr("rkt: error removing aci for imageID %q: %v\n", pkey, err)
			}
			continue
		}
		stdout("rkt: successfully removed aci for imageID: %q\n", pkey)

		// Remove the treestore only if the image isn't referenced by
		// some containers
		// TODO(sgotti) there's a windows between getting refenced
		// images and this check where a new container could be
		// prepared/runned with this image. To avoid this a global lock
		// is needed.
		if _, ok := referencedImgs[key]; ok {
			stderr("rkt: imageID is referenced by some containers, cannot remove the tree store")
			continue
		} else {
			err = s.RemoveTreeStore(key)
			if err != nil {
				staleErrors++
				stderr("rkt: error removing treestore for imageID %q: %v\n", pkey, err)
				continue
			}
		}
		errors--
		done++
	}

	if done > 0 {
		stdout("rkt: %d image(s) successfully removed\n", done)
	}
	if errors > 0 {
		stdout("rkt: %d image(s) cannot be removed\n", errors)
	}
	if staleErrors > 0 {
		stdout("rkt: %d image(s) removed but left some stale files\n", staleErrors)
	}
	return 0
}

func getReferencedImgs(s *store.Store) (map[string]struct{}, error) {
	imgs := map[string]struct{}{}
	walkErrors := []error{}
	// Consider pods in preparing, prepared, run, exitedgarbage state
	if err := walkPods(includeMostDirs, func(p *pod) {
		appImgs, err := p.getAppsHashes()
		if err != nil {
			// Ignore errors reading/parsing pod file
			return
		}
		for _, appImg := range appImgs {
			key, err := s.ResolveKey(appImg.String())
			if err != nil && err != store.ErrKeyNotFound {
				walkErrors = append(walkErrors, fmt.Errorf("bad imageID %q in pod definition: %v", appImg.String(), err))
				return
			}
			imgs[key] = struct{}{}
		}
	}); err != nil {
		return nil, fmt.Errorf("failed to get pod handles: %v", err)
	}
	if len(walkErrors) > 0 {
		return nil, fmt.Errorf("errors occured walking pods. errors: %v", walkErrors)

	}
	return imgs, nil
}
