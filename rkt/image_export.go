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
	"io"
	"os"

	"github.com/coreos/rkt/store"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/spf13/cobra"
)

var (
	cmdImageExport = &cobra.Command{
		Use:   "export IMAGE OUTPUT_ACI_FILE",
		Short: "Export a stored image to an ACI file",
		Long:  `IMAGE should be a string referencing an image: either a hash or an image name.`,
		Run:   runWrapper(runImageExport),
	}
	flagOverwriteACI bool
)

func init() {
	cmdImage.AddCommand(cmdImageExport)
	cmdImageExport.Flags().BoolVar(&flagOverwriteACI, "overwrite", false, "overwrite output ACI")
}

func runImageExport(cmd *cobra.Command, args []string) (exit int) {
	if len(args) != 2 {
		cmd.Usage()
		return 1
	}

	s, err := store.NewStore(getDataDir())
	if err != nil {
		stderr("image export: cannot open store: %v", err)
		return 1
	}

	key, err := getStoreKeyFromAppOrHash(s, args[0])
	if err != nil {
		stderr("image export: %v", err)
		return 1
	}

	aci, err := s.ReadStream(key)
	if err != nil {
		stderr("image export: error reading image: %v", err)
		return 1
	}
	defer aci.Close()

	mode := os.O_CREATE | os.O_WRONLY
	if flagOverwriteACI {
		mode |= os.O_TRUNC
	} else {
		mode |= os.O_EXCL
	}
	f, err := os.OpenFile(args[1], mode, 0644)
	if err != nil {
		if os.IsExist(err) {
			stderr("image export: output ACI file exists (try --overwrite)")
		} else {
			stderr("image export: unable to open output ACI file %s: %v", args[1], err)
		}
		return 1
	}
	defer func() {
		err := f.Close()
		if err != nil {
			stderr("image export: error closing output ACI file: %v", err)
			exit = 1
		}
	}()

	_, err = io.Copy(f, aci)
	if err != nil {
		stderr("image export: error writing to output ACI file: %v", err)
		return 1
	}

	return 0
}
