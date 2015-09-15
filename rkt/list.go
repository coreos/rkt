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
	"strings"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/schema"
	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/spf13/cobra"
	common "github.com/coreos/rkt/common"
	"github.com/coreos/rkt/networking/netinfo"
	"github.com/coreos/rkt/store"
)

var (
	cmdList = &cobra.Command{
		Use:   "list",
		Short: "List pods",
		Run:   runWrapper(runList),
	}
	flagNoLegend   bool
	flagFullOutput bool
)

func init() {
	cmdRkt.AddCommand(cmdList)
	cmdList.Flags().BoolVar(&flagNoLegend, "no-legend", false, "suppress a legend with the list")
	cmdList.Flags().BoolVar(&flagFullOutput, "full", false, "use long output format")
}

func runList(cmd *cobra.Command, args []string) (exit int) {
	s, err := store.NewStore(getDataDir())
	if err != nil {
		stderr("list: cannot open store: %v", err)
		return 1
	}

	if !flagNoLegend {
		fmt.Fprintf(tabOut, "UUID\tAPP\tACI\tSTATE\tNETWORKS\n")
	}

	if err := walkPods(includeMostDirs, func(p *pod) {
		pm := schema.PodManifest{}

		if !p.isPreparing && !p.isAbortedPrepare && !p.isExitedDeleting {
			// TODO(vc): we should really hold a shared lock here to prevent gc of the pod
			pmf, err := p.readFile(common.PodManifestPath(""))
			if err != nil {
				stderr("Unable to read pod manifest: %v", err)
				return
			}

			if err := pm.UnmarshalJSON(pmf); err != nil {
				stderr("Unable to load manifest: %v", err)
				return
			}

			if len(pm.Apps) == 0 {
				stderr("Pod contains zero apps")
				return
			}
		}

		uuid := ""
		if flagFullOutput {
			uuid = p.uuid.String()
		} else {
			uuid = p.uuid.String()[:8]
		}

		for i, app := range pm.Apps {
			// Retrieve the image from the store.
			im, err := s.GetImageManifest(app.Image.ID.String())
			if err != nil {
				stderr("Unable to load image manifest: %v", err)
			}

			if i == 0 {
				fmt.Fprintf(tabOut, "%s\t%s\t%s\t%s\t%s\n", uuid, app.Name.String(), im.Name.String(), p.getState(), fmtNets(p.nets))
			} else {
				fmt.Fprintf(tabOut, "\t%s\t%s\t\t\n", app.Name.String(), im.Name.String())
			}
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
