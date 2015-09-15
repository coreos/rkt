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

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/schema/types"
	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/spf13/cobra"
	"github.com/coreos/rkt/store"
)

var (
	cmdImageRm = &cobra.Command{
		Use:   "rm IMAGEID...",
		Short: "Remove image(s) with the given key(s) from the local store",
		Run:   runWrapper(runRmImage),
	}
)

func init() {
	cmdImage.AddCommand(cmdImageRm)
}

func runRmImage(cmd *cobra.Command, args []string) (exit int) {
	if len(args) < 1 {
		stderr("rkt: Must provide at least one image key")
		return 1
	}

	s, err := store.NewStore(getDataDir())
	if err != nil {
		stderr("rkt: cannot open store: %v", err)
		return 1
	}

	referencedImgs, err := getReferencedImgs(s)
	if err != nil {
		stderr("rkt: cannot get referenced images: %v", err)
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
			stderr("rkt: wrong imageID %q: %v", pkey, err)
			continue
		}
		key, err := s.ResolveKey(h.String())
		if err != nil {
			stderr("rkt: imageID %q not valid: %v", pkey, err)
			continue
		}
		if key == "" {
			stderr("rkt: imageID %q doesn't exists", pkey)
			continue
		}

		// Remove the image only if the image isn't referenced by
		// some containers
		// TODO(sgotti) there's a windows between getting refenced
		// images and this check where a new container could be
		// prepared/runned with this image. To avoid this a global lock
		// is needed.
		if _, ok := referencedImgs[key]; ok {
			stderr("rkt: imageID %q is referenced by some containers, cannot remove.", pkey)
			continue
		} else {
			err = s.RemoveACI(key)
			if err != nil {
				if serr, ok := err.(*store.StoreRemovalError); ok {
					staleErrors++
					stderr("rkt: some files cannot be removed for imageID %q: %v", pkey, serr)
				} else {
					stderr("rkt: error removing aci for imageID %q: %v", pkey, err)
					continue
				}
			}

			err = s.RemoveTreeStore(key)
			if err != nil {
				staleErrors++
				stderr("rkt: error removing treestore for imageID %q: %v", pkey, err)
				continue
			}
			stdout("rkt: successfully removed aci for imageID: %q", pkey)
		}
		errors--
		done++
	}

	if done > 0 {
		stderr("rkt: %d image(s) successfully removed", done)
	}
	if errors > 0 {
		stderr("rkt: %d image(s) cannot be removed", errors)
	}
	if staleErrors > 0 {
		stderr("rkt: %d image(s) removed but left some stale files", staleErrors)
	}
	return 0
}

func getReferencedImgs(s *store.Store) (map[string]struct{}, error) {
	imgs := map[string]struct{}{}
	walkErrors := []error{}
	// Consider pods in preparing, prepared, run, exitedgarbage state
	if err := walkPods(includeMostDirs, func(p *pod) {
		appHashes, err := p.getAppsHashes()
		if err != nil {
			// Ignore errors reading/parsing pod file
			return
		}
		stage1Hash, err := p.getStage1Hash()
		if err != nil {
			// Ignore errors reading/parsing the stage1 hash file
			return
		}
		allHashes := append(appHashes, *stage1Hash)

		for _, imgHash := range allHashes {
			key, err := s.ResolveKey(imgHash.String())
			if err != nil && err != store.ErrKeyNotFound {
				walkErrors = append(walkErrors, fmt.Errorf("bad imageID %q in pod definition: %v", imgHash.String(), err))
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
