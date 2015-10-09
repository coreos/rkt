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
	"os"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/octools/oci2aci"
	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/spf13/cobra"
	"github.com/coreos/rkt/store"
)

var (
	cmdImageImport = &cobra.Command{
		Use:   "import-oci OCI-BUNDLE",
		Short: "Convert imported oci-bundle to aci image",
		Long:  "Import an oci-bundle directory as input, convert to an aci image and store it in the local store",
		Run:   runWrapper(runImageImport),
	}
)

func init() {
	cmdImage.AddCommand(cmdImageImport)
}

func runImageImport(cmd *cobra.Command, args []string) (exit int) {
	if len(args) != 1 {
		cmd.Usage()
		return 1
	}

	//convert oci bundle to aci image
	aciImage, err := oci2aci.Oci2aciImage(args[0])
	if err != nil {
		fmt.Printf("oci2aci failed: %v", err)
		return 1
	}

	//save aci to rkt store
	s, err := store.NewStore(globalFlags.Dir)
	if err != nil {
		fmt.Printf("cannot open store: %v", err)
		return 1
	}
	aciFile, err := os.Open(aciImage)
	if err != nil {
		fmt.Printf("opening ACI file %s failed: %v", aciImage, err)
		return 1
	}
	key, err := s.WriteACI(aciFile, true)
	if err != nil {
		fmt.Printf("write ACI file failed: %v", err)
		return 1
	}
	fmt.Println(key)
	return 0
}
