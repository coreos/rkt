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
	"os"
)

var (
	cmdImage = &Command{
		Name:    "image",
		Summary: "Operate on an image in the local store",
		Usage:   "SUBCOMMAND IMAGE [args...]",
		Description: `SUBCOMMAND could be "cat-manifest". IMAGE should be a string referencing an image; either a hash, local file on disk, or URL.
They will be checked in that order and the first match will be used.`,
		Run:   runImage,
		Flags: &imageFlags,
	}
	imageFlags flag.FlagSet
)

func init() {
	commands = append(commands, cmdImage)
}

func runImage(args []string) (exit int) {
	if len(args) < 1 {
		printCommandUsageByName("image")
		return 1
	}

	var subCmd *Command
	subArgs := args[1:]

	// determine which Command should be run
	for _, c := range subCommands["image"] {
		if c.Name == args[0] {
			subCmd = c
			if err := c.Flags.Parse(subArgs); err != nil {
				stderr("%v", err)
				os.Exit(2)
			}
			break
		}
	}

	if subCmd == nil {
		stderr("image: unknown subcommand: %q", args[0])
		os.Exit(2)
	}
	return subCmd.Run(subArgs[subCmd.Flags.NFlag():])
}
