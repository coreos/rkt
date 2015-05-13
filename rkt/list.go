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

//+build linux

package main

import (
	"fmt"
	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/spf13/cobra"
	"strings"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/schema"
	common "github.com/coreos/rkt/common"
	"github.com/coreos/rkt/networking/netinfo"
)

var (
	cmdList = &cobra.Command{
		Use:   "list",
		Short: "List pods",
		Run:   func(cmd *cobra.Command, args []string) { subCmdExitCode = runList(cmd, args) },
	}
	flagNoLegend   bool
	flagFullOutput bool
)

func init() {
	cmdList.Flags().BoolVarP(&flagNoLegend, "no-legend", "n", false, "suppress a legend with the list")
	cmdList.Flags().BoolVarP(&flagFullOutput, "full", "", false, "use long output format")
	rktCmd.AddCommand(cmdList)
}

func runList(cmd *cobra.Command, args []string) (exit int) {
	if !flagNoLegend {
		fmt.Fprintf(tabOut, "UUID\tACI\tSTATE\tNETWORKS\n")
	}

	if err := walkPods(includeMostDirs, func(p *pod) {
		m := schema.PodManifest{}
		app_zero := ""

		if !p.isPreparing && !p.isAbortedPrepare && !p.isExitedDeleting {
			// TODO(vc): we should really hold a shared lock here to prevent gc of the pod
			manifFile, err := p.readFile(common.PodManifestPath(""))
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
				stderr("Pod contains zero apps")
				return
			}
			app_zero = m.Apps[0].Name.String()
		}

		uuid := ""
		if flagFullOutput {
			uuid = p.uuid.String()
		} else {
			uuid = p.uuid.String()[:8]
		}

		fmt.Fprintf(tabOut, "%s\t%s\t%s\t%s\n", uuid, app_zero, p.getState(), fmtNets(p.nets))
		for i := 1; i < len(m.Apps); i++ {
			fmt.Fprintf(tabOut, "\t%s\n", m.Apps[i].Name.String())
		}
	}); err != nil {
		stderr("Failed to get pod handles: %v", err)
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
