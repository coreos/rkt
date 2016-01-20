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

//+build linux

package main

import (
	"fmt"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/spf13/cobra"
)

var (
	cmdStatus = &cobra.Command{
		Use:   "status [--wait] UUID",
		Short: "Check the status of a rkt pod",
		Run:   runWrapper(runStatus),
	}
	flagWait bool
)

const (
	overlayStatusDirTemplate = "overlay/%s/upper/rkt/status"
	regularStatusDir         = "stage1/rootfs/rkt/status"
	cmdStatusName            = "status"
)

func init() {
	cmdRkt.AddCommand(cmdStatus)
	cmdStatus.Flags().BoolVar(&flagWait, "wait", false, "toggle waiting for the pod to exit")
}

func runStatus(cmd *cobra.Command, args []string) (exit int) {
	if len(args) != 1 {
		cmd.Usage()
		return 1
	}

	p, err := getPodFromUUIDString(args[0])
	if err != nil {
		stderr("Problem retrieving pod: %v", err)
		return 1
	}
	defer p.Close()

	if flagWait {
		if err := p.waitExited(); err != nil {
			stderr("Unable to wait for pod: %v", err)
			return 1
		}
	}

	if err = printStatus(p); err != nil {
		stderr("Unable to print status: %v", err)
		return 1
	}

	return 0
}

// printStatus prints the pod's pid and per-app status codes
func printStatus(p *pod) error {
	stdout("state=%s", p.getState())

	created, err := p.getRootModTime()
	if err != nil {
		return fmt.Errorf("error getting pod root modification time: %v", err)
	}
	exited, err := p.getExitTime()
	if err != nil {
		return fmt.Errorf("error getting pod exit time: %v", err)
	}

	var since string
	if p.isExited || p.isExitedGarbage {
		if !exited.IsZero() {
			since = exited.Format(defaultTimeLayout)
		}
	} else if !created.IsZero() {
		since = created.Format(defaultTimeLayout)
	}

	stdout("since=%s", since)

	if p.isRunning() {
		stdout("networks=%s", fmtNets(p.nets))
	}

	if !p.isEmbryo && !p.isPreparing && !p.isPrepared && !p.isAbortedPrepare && !p.isGarbage && !p.isGone {
		pid, err := p.getPID()
		if err != nil {
			return err
		}

		stats, err := p.getExitStatuses()
		if err != nil {
			return err
		}

		stdout("pid=%d\nexited=%t", pid, p.isExited)
		for app, stat := range stats {
			stdout("app-%s=%d", app, stat)
		}
	}
	return nil
}
