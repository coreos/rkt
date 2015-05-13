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
	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/spf13/cobra"
)

var (
	cmdImage *cobra.Command
)

func init() {
	cmdImage = &cobra.Command{
		Use:   "image SUBCOMMAND IMAGE [args...]",
		Short: "Operate on an image in the local store",
		Long: `SUBCOMMAND could be "cat-manifest". IMAGE should be a string referencing an image; either a hash, local file on disk, or URL.
They will be checked in that order and the first match will be used.`,
		Run: func(cmd *cobra.Command, args []string) {
			subCmdExitCode = runImage(cmdImage, args)
		},
	}
	rktCmd.AddCommand(cmdImage)
}

func runImage(cmd *cobra.Command, args []string) (exit int) {
	cmd.Help()
	return 1
}
