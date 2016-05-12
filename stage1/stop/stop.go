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
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"syscall"
)

var (
	force bool
)

func init() {
	flag.BoolVar(&force, "force", false, "Forced stopping")
}

func readIntFromFile(path string) (i int, err error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return
	}
	_, err = fmt.Sscanf(string(b), "%d", &i)
	return
}

func stop(signal syscall.Signal) int {
	pid, err := readIntFromFile("ppid")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading pid: %v\n", err)
		return 1
	}

	if err := syscall.Kill(pid, signal); err != nil {
		fmt.Fprintf(os.Stderr, "error sending %v: %v\n", signal, err)
		return 1
	}

	return 0
}

func main() {
	flag.Parse()

	signal := syscall.SIGTERM
	if force {
		signal = syscall.SIGKILL
	}

	os.Exit(stop(signal))
}
