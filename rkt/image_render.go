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
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/coreos/rkt/pkg/fileutil"
	"github.com/coreos/rkt/pkg/uid"
	"github.com/coreos/rkt/store"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/spf13/cobra"
)

var (
	cmdImageRender = &cobra.Command{
		Use:   "render IMAGE OUTPUT_DIR",
		Short: "Render a stored image to a directory with all its dependencies",
		Long:  `IMAGE should be a string referencing an image: either a hash or an image name.`,
		Run:   runWrapper(runImageRender),
	}
	flagRenderRootfsOnly bool
	flagRenderOverwrite  bool
)

func init() {
	cmdImage.AddCommand(cmdImageRender)
	cmdImageRender.Flags().BoolVar(&flagRenderRootfsOnly, "rootfs-only", false, "render rootfs only")
	cmdImageRender.Flags().BoolVar(&flagRenderOverwrite, "overwrite", false, "overwrite output directory")
}

func runImageRender(cmd *cobra.Command, args []string) (exit int) {
	if len(args) != 2 {
		cmd.Usage()
		return 1
	}
	outputDir := args[1]

	s, err := store.NewStore(getDataDir())
	if err != nil {
		stderr("image render: cannot open store: %v", err)
		return 1
	}

	key, err := getStoreKeyFromAppOrHash(s, args[0])
	if err != nil {
		stderr("image render: %v", err)
		return 1
	}

	if err := s.RenderTreeStore(key, false); err != nil {
		stderr("image render: error rendering ACI: %v", err)
		return 1
	}
	if err := s.CheckTreeStore(key); err != nil {
		stderr("image render: warning: tree cache is in a bad state. Rebuilding...")
		if err := s.RenderTreeStore(key, true); err != nil {
			stderr("image render: error rendering ACI: %v", err)
			return 1
		}
	}

	if _, err := os.Stat(outputDir); err == nil {
		if !flagRenderOverwrite {
			stderr("image render: output directory exists (try --overwrite)")
			return 1
		}

		// don't allow the user to delete the root filesystem by mistake
		if outputDir == "/" {
			stderr("image extract: this would delete your root filesystem. Refusing.")
			return 1
		}

		if err := os.RemoveAll(outputDir); err != nil {
			stderr("image render: error removing existing output dir: %v", err)
			return 1
		}
	}
	rootfsOutDir := outputDir
	if !flagRenderRootfsOnly {
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			stderr("image render: error creating output directory: %v", err)
			return 1
		}
		rootfsOutDir = filepath.Join(rootfsOutDir, "rootfs")

		manifest, err := s.GetImageManifest(key)
		if err != nil {
			stderr("image render: error getting manifest: %v", err)
			return 1
		}

		mb, err := json.Marshal(manifest)
		if err != nil {
			stderr("image render: error marshalling image manifest: %v", err)
			return 1
		}

		if err := ioutil.WriteFile(filepath.Join(outputDir, "manifest"), mb, 0700); err != nil {
			stderr("image render: error writing image manifest: %v", err)
			return 1
		}
	}

	cachedTreePath := s.GetTreeStoreRootFS(key)
	if err := fileutil.CopyTree(cachedTreePath, rootfsOutDir, uid.NewBlankUidRange()); err != nil {
		stderr("image render: error copying ACI rootfs: %v", err)
		return 1
	}

	return 0
}
