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
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"

	"github.com/coreos/rkt/pkg/fileutil"
	"github.com/coreos/rkt/pkg/tar"
	"github.com/coreos/rkt/pkg/uid"
	"github.com/coreos/rkt/store"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/spf13/cobra"
)

var (
	cmdImageExtract = &cobra.Command{
		Use:   "extract IMAGE OUTPUT_DIR",
		Short: "Extract a stored image to a directory",
		Long:  `IMAGE should be a string referencing an image: either a hash or an image name.`,
		Run:   runWrapper(runImageExtract),
	}
	flagExtractRootfsOnly bool
	flagExtractOverwrite  bool
)

func init() {
	cmdImage.AddCommand(cmdImageExtract)
	cmdImageExtract.Flags().BoolVar(&flagExtractRootfsOnly, "rootfs-only", false, "extract rootfs only")
	cmdImageExtract.Flags().BoolVar(&flagExtractOverwrite, "overwrite", false, "overwrite output directory")
}

func runImageExtract(cmd *cobra.Command, args []string) (exit int) {
	if len(args) != 2 {
		cmd.Usage()
		return 1
	}
	outputDir := args[1]

	s, err := store.NewStore(getDataDir())
	if err != nil {
		stderr("image extract: cannot open store: %v", err)
		return 1
	}

	key, err := getStoreKeyFromAppOrHash(s, args[0])
	if err != nil {
		stderr("image extract: %v", err)
		return 1
	}

	aci, err := s.ReadStream(key)
	if err != nil {
		stderr("image extract: error reading ACI from the store: %v", err)
		return 1
	}

	// ExtractTar needs an absolute path
	absOutputDir, err := filepath.Abs(outputDir)
	if err != nil {
		stderr("image extract: error converting output to an absolute path: %v", err)
		return 1
	}

	if _, err := os.Stat(absOutputDir); err == nil {
		if !flagExtractOverwrite {
			stderr("image extract: output directory exists (try --overwrite)")
			return 1
		}

		// don't allow the user to delete the root filesystem by mistake
		if absOutputDir == "/" {
			stderr("image extract: this would delete your root filesystem. Refusing.")
			return 1
		}

		if err := os.RemoveAll(absOutputDir); err != nil {
			stderr("image extract: error removing existing output dir: %v", err)
			return 1
		}
	}

	// if the user only asks for the rootfs we extract the image to a temporary
	// directory and then move/copy the rootfs to the output directory, if not
	// we just extract the image to the output directory
	extractDir := absOutputDir
	if flagExtractRootfsOnly {
		rktTmpDir, err := s.TmpDir()
		if err != nil {
			stderr("image extract: error creating rkt temporary directory: %v", err)
			return 1
		}

		tmpDir, err := ioutil.TempDir(rktTmpDir, "rkt-image-extract-")
		if err != nil {
			stderr("image extract: error creating temporary directory: %v", err)
			return 1
		}
		defer os.RemoveAll(tmpDir)

		extractDir = tmpDir
	} else {
		if err := os.MkdirAll(absOutputDir, 0755); err != nil {
			stderr("image extract: error creating output directory: %v", err)
			return 1
		}
	}

	if err := tar.ExtractTar(aci, extractDir, false, uid.NewBlankUidRange(), nil); err != nil {
		stderr("image extract: error extracting ACI: %v", err)
		return 1
	}

	if flagExtractRootfsOnly {
		rootfsDir := filepath.Join(extractDir, "rootfs")
		if err := os.Rename(rootfsDir, absOutputDir); err != nil {
			if e, ok := err.(*os.LinkError); ok && e.Err == syscall.EXDEV {
				// it's on a different device, fall back to copying
				if err := fileutil.CopyTree(rootfsDir, absOutputDir, uid.NewBlankUidRange()); err != nil {
					stderr("image extract: error copying ACI rootfs: %v", err)
					return 1
				}
			} else {
				stderr("image extract: error moving ACI rootfs: %v", err)
				return 1
			}
		}
	}

	return 0
}
