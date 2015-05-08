// Copyright 2014 The rkt Authors
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
	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/schema"
	"github.com/coreos/rkt/version"
)

var (
	cmdVersion = &Command{
		Name:        "version",
		Description: "Print the version and exit",
		Summary:     "Print the version and exit",
		Run:         runVersion,
		Flags:       &versionFlags,
	}
	versionFlags flag.FlagSet
)

func init() {
	commands = append(commands, cmdVersion)
}

func runVersion(args []string) (exit int) {
	stdout("rkt version %s", version.Version)
	stdout("appc version %s", schema.AppContainerVersion)
	return
}
