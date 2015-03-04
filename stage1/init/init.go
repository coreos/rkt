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

// this implements /init of stage1/nspawn+systemd

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/coreos/rocket/common"
	"github.com/coreos/rocket/networking"
)

const (
	// Path to capsule binary within the stage1 rootfs
	capsuleBin = "/capsule"
	// Path to systemd binary within the stage1 rootfs
	systemdBin = "/usr/lib/systemd/systemd"
)

var (
	debug   bool
	privNet bool
)

func init() {
	flag.BoolVar(&debug, "debug", false, "Run in debug mode")
	flag.BoolVar(&privNet, "private-net", false, "Setup private network (WIP!)")

	// this ensures that main runs only on main thread (thread group leader).
	// since namespace ops (unshare, setns) are done for a single thread, we
	// must ensure that the goroutine does not jump from OS thread to thread
	runtime.LockOSThread()
}

func stage1() int {
	root := "."
	c, err := LoadContainer(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load container: %v\n", err)
		return 1
	}

	c.MetadataSvcURL = common.MetadataSvcPublicURL()

	if err = c.ContainerToSystemd(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to configure systemd: %v\n", err)
		return 2
	}

	args := []string{
		filepath.Join(common.Stage1RootfsPath(c.Root), capsuleBin),
	}

	if !debug {
		args = append(args, "--quiet") // silence most capsule output (log_warning is currently not covered by this)
	}

	nsargs, err := c.ContainerToCapsuleArgs()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate capsule args: %v\n", err)
		return 4
	}
	args = append(args, nsargs...)

	// Arguments to systemd
	args = append(args, "--", systemdBin)
	args = append(args, "--default-standard-output=tty") // redirect all service logs straight to tty
	if !debug {
		args = append(args, "--log-target=null") // silence systemd output inside container
		args = append(args, "--show-status=0")   // silence systemd initialization status output
	}

	env := os.Environ()

	if privNet {
		// careful not to make another local err variable.
		// cmd.Run sets the one from parent scope
		var n *networking.Networking
		n, err = networking.Setup(root, c.Manifest.UUID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to setup network: %v\n", err)
			return 6
		}
		defer n.Teardown()

		if err = n.EnterContNS(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to switch to container netns: %v\n", err)
			return 6
		}

		if err = registerContainer(c, n.MetadataIP); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to register container: %v\n", err)
			return 6
		}
		defer unregisterContainer(c)

		cmd := exec.Cmd{
			Path:   args[0],
			Args:   args,
			Stdin:  os.Stdin,
			Stdout: os.Stdout,
			Stderr: os.Stderr,
			Env:    env,
		}
		err = cmd.Run()
	} else {
		err = syscall.Exec(args[0], args, env)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to execute nspawn: %v\n", err)
		return 5
	}

	return 0
}

func main() {
	flag.Parse()
	// move code into stage1() helper so defered fns get run
	os.Exit(stage1())
}
