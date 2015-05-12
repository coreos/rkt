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
	"flag"
)

var (
	cmdStatus = &Command{
		Name:    cmdStatusName,
		Summary: "Check the status of a rkt pod",
		Usage:   "[--wait] UUID",
		Run:     runStatus,
		Flags:   &statusFlags,
	}
	statusFlags flag.FlagSet
	flagWait    bool
)

const (
	overlayStatusDirTemplate = "overlay/%s/upper/rkt/status"
	regularStatusDir         = "stage1/rootfs/rkt/status"
	cmdStatusName            = "status"
)

func init() {
	commands = append(commands, cmdStatus)
	statusFlags.BoolVar(&flagWait, "wait", false, "toggle waiting for the pod to exit")
}

func runStatus(args []string) (exit int) {
	if len(args) != 1 {
		printCommandUsageByName(cmdStatusName)
		return 1
	}

	podUUID, err := resolveUUID(args[0])
	if err != nil {
		stderr("Unable to resolve UUID: %v", err)
		return 1
	}

	p, err := getPod(podUUID.String())
	if err != nil {
		stderr("Unable to get pod: %v", err)
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

	if p.isRunning() {
		stdout("networks=%s", fmtNets(p.nets))
	}

	if !p.isEmbryo && !p.isPreparing && !p.isPrepared && !p.isAbortedPrepare && !p.isGarbage && !p.isGone {
		pid, err := p.getPID()
		if err != nil {
			return err
		}

		if p.isExited {
			stats, err := p.getExitStatuses()
			if err != nil {
				return err
			}

			stdout("pid=%d\nexited=%t", pid, p.isExited)
			for app, stat := range stats {
				stdout("%s=%d", app, stat)
			}
		}
	}
	return nil
}
