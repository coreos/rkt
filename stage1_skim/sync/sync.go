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

// Program used to orchestrate w/ other apps in the pod, aggregate the output
// and dump things out to stdout
// Accept a single argument to reflect the service name of the first

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

func dumpOutput(f *os.File) {
	for {
		_, err := io.Copy(os.Stdout, f)
		if (err != nil) && (err == io.EOF) {
			break
		} else if err != nil {
			fmt.Printf("Error encountered while reading from fifo: %q\n", err)
			os.Exit(254)
		}
	}
}

func cleanup() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGUSR1)
	_ = <-c

	os.Exit(0)
}

func main() {
	if len(os.Args) != 3 {
		fmt.Printf("ERROR: Expecting the service to start as well as the pod basedir")
		os.Exit(254)
	}

	/* Setup the fifo queue */
	serverFifo := filepath.Join(os.Args[2], "sync.fifo")
	err := syscall.Mkfifo(serverFifo, syscall.S_IRUSR|syscall.S_IWUSR|syscall.S_IWGRP)
	if err != nil {
		fmt.Printf("ERROR: Cannot create %s: %q\n", serverFifo, err)
		os.Exit(254)
	}

	/* Get everything ready */
	systemctlCmd := "/usr/bin/systemctl"
	systemctlArgs := []string{systemctlCmd, "start", os.Args[1]}
	go cleanup()

	_, err = syscall.ForkExec(systemctlCmd, systemctlArgs, nil)
	if err != nil {
		fmt.Printf("ERROR: Unable to start service: %q\n", err)
		os.Exit(254)
	}

	fd, err := os.Open(serverFifo)
	if err != nil {
		fmt.Printf("ERROR: Cannot open %s: %q\n", serverFifo, err)
		os.Exit(254)
	}

	defer fd.Close()

	dumpOutput(fd)
}
