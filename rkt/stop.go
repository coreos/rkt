// Copyright 2016 The rkt Authors
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

	"github.com/coreos/rkt/stage0"

	"github.com/appc/spec/schema/types"
	"github.com/spf13/cobra"
)

var (
	cmdStop = &cobra.Command{
		Use:   "stop --uuid-file=FILE | UUID ...",
		Short: "Stop a pod",
		Run:   runWrapper(runStop),
	}
	flagForce bool
)

func init() {
	cmdRkt.AddCommand(cmdStop)
	cmdStop.Flags().BoolVar(&flagForce, "force", false, "forced stopping")
	cmdStop.Flags().StringVar(&flagUUIDFile, "uuid-file", "", "read pod UUID from file instead of argument")
}

func runStop(cmd *cobra.Command, args []string) (exit int) {
	var podUUID *types.UUID
	var podUUIDs []*types.UUID
	var err error
	var errors int

	ret := 0
	switch {
	case len(args) == 0 && flagUUIDFile != "":
		podUUID, err = readUUIDFromFile(flagUUIDFile)
		if err != nil {
			stderr.PrintE("unable to resolve UUID from file", err)
			ret = 1
		} else {
			podUUIDs = append(podUUIDs, podUUID)
		}

	case len(args) > 0 && flagUUIDFile == "":
		for _, uuid := range args {
			podUUID, err := resolveUUID(uuid)
			if err != nil {
				stderr.PrintE("unable to resolve UUID", err)
				ret = 1
			} else {
				podUUIDs = append(podUUIDs, podUUID)
			}
		}

	default:
		cmd.Usage()
		return 1
	}

	for _, podUUID = range podUUIDs {
		p, err := getPod(podUUID)
		if err != nil {
			errors++
			stderr.PrintE("cannot get pod", err)
		}

		if !p.isRunning() {
			stderr.Error(fmt.Errorf("pod %q is not running", p.uuid))
			errors++
			continue
		}

		if err := stage0.StopPod(p.path(), flagForce, podUUID); err == nil {
			stdout.Printf("%q", p.uuid)
		} else {
			stderr.PrintE(fmt.Sprintf("error stopping %q", p.uuid), err)
			errors++
		}
	}

	if errors > 0 {
		stderr.Error(fmt.Errorf("failed to stop %d pod(s)", errors))
		return 1
	}

	return ret
}
