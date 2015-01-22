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
	"os"

	"github.com/appc/spec/schema/types"
)

var (
	cmdListContainers = &Command{
		Name:    "list-containers",
		Summary: "List rkt containers and their status",
		Usage:   "UUID",
		Run:     runListContainers,
	}
)

func init() {
	commands = append(commands, cmdListContainers)
}

func runListContainers(args []string) (exit int) {
	cs, err := getContainers()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to get containers list: %v\n", err)
		return 1
	}
	fmt.Fprintf(out, "CONTAINER\tSTATUS\n")
	for _, c := range cs {
		containerUUID, err := types.NewUUID(c)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid UUID: %v\n", err)
			return 1
		}

		l, exited, err := getContainerLockAndState(containerUUID, false)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to access container: %v\n", err)
			continue
		}
		defer l.Close()

		if exited {
			fmt.Fprintf(out, "%s\texited\n", c)
		} else {
			fmt.Fprintf(out, "%s\trunning\n", c)
		}
	}
	out.Flush()

	return
}
