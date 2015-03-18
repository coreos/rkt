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
	"strings"

	"github.com/coreos/rocket/Godeps/_workspace/src/github.com/appc/spec/schema"
	common "github.com/coreos/rocket/common"
	"github.com/coreos/rocket/networking/netinfo"
)

var (
	cmdList = &Command{
		Name:    "list",
		Summary: "List containers",
		Usage:   "",
		Run:     runList,
	}
	flagNoLegend   bool
	flagLongOutput bool
)

func init() {
	commands = append(commands, cmdList)
	cmdList.Flags.BoolVar(&flagNoLegend, "no-legend", false, "suppress a legend with the list")
	cmdList.Flags.BoolVar(&flagLongOutput, "long", false, "use long output format")
}

func runList(args []string) (exit int) {
	if !flagNoLegend {
		fmt.Fprintf(tabOut, "UUID\tACI\tSTATE\tNETWORKS\n")
	}

	if err := walkContainers(includeMostDirs, func(c *container) {
		m := schema.ContainerRuntimeManifest{}
		app_zero := ""

		if !c.isPreparing && !c.isAbortedPrepare && !c.isExitedDeleting {
			// TODO(vc): we should really hold a shared lock here to prevent gc of the container
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

		uuid := ""
		if flagLongOutput {
			uuid = c.uuid.String()
		} else {
			uuid = c.uuid.String()[:8]
		}

		fmt.Fprintf(tabOut, "%s\t%s\t%s\t%s\n", uuid, app_zero, c.getState(), fmtNets(c.nets))
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

func fmtNets(nis []netinfo.NetInfo) string {
	parts := []string{}
	for _, ni := range nis {
		// there will be IPv6 support soon so distinguish between v4 and v6
		parts = append(parts, fmt.Sprintf("%v:ip4=%v", ni.NetName, ni.IP))
	}
	return strings.Join(parts, ", ")
}
