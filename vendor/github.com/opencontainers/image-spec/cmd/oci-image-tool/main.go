// Copyright 2016 The Linux Foundation
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
	"log"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	cmd := &cobra.Command{
		Use:   "oci-image-tool",
		Short: "A tool for working with OCI images",
	}

	stdout := log.New(os.Stdout, "", 0)
	stderr := log.New(os.Stderr, "", 0)

	cmd.AddCommand(newValidateCmd(stdout, stderr))
	cmd.AddCommand(newUnpackCmd(stdout, stderr))
	cmd.AddCommand(newBundleCmd(stdout, stderr))

	if err := cmd.Execute(); err != nil {
		stderr.Println(err)
		os.Exit(1)
	}
}
