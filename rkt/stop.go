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
	cmdStop = &Command{
		Name:    cmdStopName,
		Summary: "Stop a rkt container",
		Usage:   "UUID",
		Run:     runStop,
	}
)

const (
	cmdStopName = "stop"
)

func init() {
	commands = append(commands, cmdStop)
}

func runStop(args []string) (exit int) {
	if len(args) != 1 {
		printCommandUsageByName(cmdStopName)
		return 1
	}

	containerUUID, err := resolveUUID(args[0])
	if err != nil {
		stderr("Unable to resolve UUID: %v", err)
		return 1
	}

	ch, err := getContainer(containerUUID.String())
	if err != nil {
		stderr("Unable to get container handle: %v", err)
		return 1
	}
	defer ch.Close()

	if err = stopContainer(ch); err != nil {
		stderr("Unable to stop container: %v", err)
		return 1
	}

	return 0
}

//stopContainer stops the container
func stopContainer(ch *container) error {
	if err := ch.stop(); err != nil {
		return err
	}

	if err := ch.waitExited(); err != nil {
		return err
	}

	return nil
}
