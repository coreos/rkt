// Copyright 2015 CoreOS, Inc.
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

//+build linux

package main

import (
	"fmt"

	"github.com/coreos/rocket/Godeps/_workspace/src/github.com/appc/spec/schema"
	common "github.com/coreos/rocket/common"
)

var (
	cmdList = &Command{
		Name:    "list",
		Summary: "List containers",
		Usage:   "",
		Run:     runList,
	}
	flagNoLegend bool
)

func init() {
	commands = append(commands, cmdList)
	cmdList.Flags.BoolVar(&flagNoLegend, "no-legend", false, "suppress a legend with the list")
}

func runList(args []string) (exit int) {
	if !flagNoLegend {
		fmt.Fprintf(tabOut, "UUID\tACI\tSTATE\n")
	}

	if err := walkContainers(includeAllDirs, func(c *container) {
		m := schema.ContainerRuntimeManifest{}
		app_zero := ""
		if !c.isPreparing {
			manifFile, err := c.readFile(common.ContainerManifestPath(""))
			if err != nil {
				stderr("Unable to read manifest: %v", err)
				return
			}

			err = m.UnmarshalJSON(manifFile)
			if err != nil {
				stderr("Unable to load manifest: %v", err)
				return
			}

			if len(m.Apps) == 0 {
				stderr("Container contains zero apps")
				return
			}
			app_zero = m.Apps[0].Name.String()
		}

		fmt.Fprintf(tabOut, "%s\t%s\t%s\n", c.uuid, app_zero, c.getState())
		for i := 1; i < len(m.Apps); i++ {
			fmt.Fprintf(tabOut, "\t%s\n", m.Apps[i].Name.String())
		}
	}); err != nil {
		stderr("Failed to get container handles: %v", err)
		return 1
	}

	tabOut.Flush()
	return 0
}
