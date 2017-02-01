// Copyright 2017 The rkt Authors
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

package main

// Program used to orchestrate w/ other apps in the pod.  Accept a single
// argument to reflect the service name of the first

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Printf("ERROR: Expecting the service to start")
		os.Exit(254)
	}

	systemctlCmd := "/usr/bin/systemctl"
	systemctlArgs := []string{systemctlCmd, "start", os.Args[1]}

	pid, err := syscall.ForkExec(systemctlCmd, systemctlArgs, nil)
	if err != nil {
		fmt.Printf("ERROR: Unable to start service: %q\n", err)
		os.Exit(254)
	}

	fmt.Printf("Starting %s (%d)\n", os.Args[1], pid)

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT)
	_ = <-c
}
