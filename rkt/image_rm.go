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

	"github.com/coreos/rkt/store/casref/rwcasref"
	"github.com/spf13/cobra"
)

var (
	cmdImageRm = &cobra.Command{
		Use:   "rm IMAGE...",
		Short: "Remove image(s) with the given digest(s) from the local store",
		Long:  `Unlike image gc, image rm allows users to remove specific images.`,
		Run:   runWrapper(runRmImage),
	}
)

func init() {
	cmdImage.AddCommand(cmdImageRm)
}

func rmImages(s *rwcasref.Store, images []string) error {
	done := 0
	errors := 0
	staleErrors := 0
	imageMap := make(map[string]struct{})
	imageCounter := make(map[string]int)

	for _, pdigest := range images {
		errors++
		digest, err := s.ResolveDigest(pdigest)
		if err != nil && err == rwcasref.ErrDigestNotFound {
			stderr.Printf("digest %q doesn't exist", pdigest)
			continue
		}
		if err != nil {
			stderr.PrintE(fmt.Sprintf("digest %q not valid", pdigest), err)
			continue
		}
		imageMap[digest] = struct{}{}
		imageCounter[digest]++
	}

	// Adjust the error count by subtracting duplicate IDs from it,
	// therefore allowing only one error per ID.
	for _, c := range imageCounter {
		if c > 1 {
			errors -= c - 1
		}
	}

	for digest := range imageMap {
		if err := s.RemoveBlob(digest, true); err != nil {
			if err == rwcasref.ErrStaleData {
				staleErrors++
				stderr.PrintE(fmt.Sprintf("some files cannot be removed for blob %q", digest), err)
			} else {
				stderr.PrintE(fmt.Sprintf("error removing blob %q", digest), err)
				continue
			}
		}
		stdout.Printf("successfully removed aci for image: %q", digest)
		errors--
		done++
	}

	if done > 0 {
		stderr.Printf("%d image(s) successfully removed", done)
	}

	// If anything didn't complete, return exit status of 1
	if (errors + staleErrors) > 0 {
		if staleErrors > 0 {
			stderr.Printf("%d image(s) removed but left some stale files", staleErrors)
		}
		if errors > 0 {
			stderr.Printf("%d image(s) cannot be removed", errors)
		}
		return fmt.Errorf("error(s) found while removing images")
	}

	return nil
}

func runRmImage(cmd *cobra.Command, args []string) (exit int) {
	if len(args) < 1 {
		stderr.Print("must provide at least one image ID")
		return 1
	}

	s, err := rwcasref.NewStore(storeDir())
	if err != nil {
		stderr.PrintE("cannot open store", err)
		return 1
	}

	if err := rmImages(s, args); err != nil {
		stderr.Error(err)
		return 1
	}

	return 0
}
