// Copyright 2014 CoreOS, Inc.
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

var (
	cmdStatus = &Command{
		Name:    cmdStatusName,
		Summary: "Check the status of a rkt container",
		Usage:   "[--wait] UUID",
		Run:     runStatus,
	}
	flagWait bool
)

const (
	statusDir     = "stage1/rootfs/rkt/status"
	cmdStatusName = "status"
)

func init() {
	commands = append(commands, cmdStatus)
	cmdStatus.Flags.BoolVar(&flagWait, "wait", false, "toggle waiting for the container to exit")
}

func runStatus(args []string) (exit int) {
	if len(args) != 1 {
		printCommandUsageByName(cmdStatusName)
		return 1
	}

	containerUUID, err := resolveUUID(args[0])
	if err != nil {
		stderr("Unable to resolve UUID: %v", err)
		return 1
	}

	c, err := getContainer(containerUUID.String())
	if err != nil {
		stderr("Unable to get container: %v", err)
		return 1
	}
	defer c.Close()

	if flagWait {
		if err := c.waitExited(); err != nil {
			stderr("Unable to wait for container: %v", err)
			return 1
		}
	}

	if err = printStatus(c); err != nil {
		stderr("Unable to print status: %v", err)
		return 1
	}

	return 0
}

// printStatus prints the container's pid and per-app status codes
func printStatus(c *container) error {
	stdout("state=%s", c.getState())

	if c.isRunning() {
		stdout("networks=%s", fmtNets(c.nets))
	}

	if !c.isEmbryo && !c.isPreparing && !c.isPrepared && !c.isAbortedPrepare && !c.isGarbage && !c.isGone {
		pid, err := c.getPID()
		if err != nil {
			return err
		}

		stats, err := c.getExitStatuses()
		if err != nil {
			return err
		}

		stdout("pid=%d\nexited=%t", pid, c.isExited)
		for app, stat := range stats {
			stdout("app=%s=%d", app.String(), stat)
		}
	}
	return nil
}
