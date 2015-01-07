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
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/coreos/rocket/metadata"
	"github.com/coreos/rocket/network"
	"github.com/coreos/rocket/path"
)

const (
	// Path to systemd-nspawn binary within the stage1 rootfs
	nspawnBin = "/usr/bin/systemd-nspawn"
	// Path to the interpreter within the stage1 rootfs
	interpBin = "/usr/lib/ld-linux-x86-64.so.2"
)

// mirrorLocalZoneInfo tries to reproduce the /etc/localtime target in stage1/ to satisfy systemd-nspawn
func mirrorLocalZoneInfo(root string) {
	zif, err := os.Readlink("/etc/localtime")
	if err != nil {
		return
	}

	src, err := os.Open(zif)
	if err != nil {
		return
	}
	defer src.Close()

	destp := filepath.Join(path.Stage1RootfsPath(root), zif)

	if err = os.MkdirAll(filepath.Dir(destp), 0755); err != nil {
		return
	}

	dest, err := os.OpenFile(destp, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer dest.Close()

	_, _ = io.Copy(dest, src)
}

var (
	debug       bool
	metadataSvc string
	privNet     bool
)

func init() {
	flag.BoolVar(&debug, "debug", false, "Run in debug mode")
	flag.StringVar(&metadataSvc, "metadata-svc", "", "Launch specified metadata svc")
	flag.BoolVar(&privNet, "priv-net", false, "Setup private network (WIP!)")

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

	mirrorLocalZoneInfo(c.Root)
	c.MetadataSvcURL = metadata.MetadataSvcPubURL()

	if err = c.ContainerToSystemd(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to configure systemd: %v\n", err)
		return 2
	}

	args := []string{
		filepath.Join(path.Stage1RootfsPath(c.Root), interpBin),
		filepath.Join(path.Stage1RootfsPath(c.Root), nspawnBin),
		"--boot",              // Launch systemd in the container
		"--register", "false", // We cannot assume the host system is running systemd
	}

	if !debug {
		args = append(args, "--quiet") // silence most nspawn output (log_warning is currently not covered by this)
	}

	nsargs, err := c.ContainerToNspawnArgs()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate nspawn args: %v\n", err)
		return 4
	}
	args = append(args, nsargs...)

	// Arguments to systemd
	args = append(args, "--")
	args = append(args, "--default-standard-output=tty") // redirect all service logs straight to tty
	if !debug {
		args = append(args, "--log-target=null") // silence systemd output inside container
		args = append(args, "--show-status=0")   // silence systemd initialization status output
	}

	env := os.Environ()
	env = append(env, "LD_PRELOAD="+filepath.Join(path.Stage1RootfsPath(c.Root), "fakesdboot.so"))
	env = append(env, "LD_LIBRARY_PATH="+filepath.Join(path.Stage1RootfsPath(c.Root), "usr/lib"))

	if metadataSvc != "" {
		if err = launchMetadataSvc(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to launch metadata svc: %v\n", err)
			return 6
		}
	}

	if privNet {
		ip, netns, err := network.Setup(c.Manifest.UUID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to setup network: %v\n", err)
			return 7
		}
		defer network.Teardown(c.Manifest.UUID, ip)

		if err = registerContainer(c, ip); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to register container: %v\n", err)
			return 6
		}
		defer unregisterContainer(c)

		if err = network.Enter(netns); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to switch container netns: %v\n", err)
			return 7
		}

		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Env = env
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to execute nspawn: %v\n", err)
			return 5
		}
	} else {
		if err := syscall.Exec(args[0], args, env); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to execute nspawn: %v\n", err)
			os.Exit(5)
		}
	}

	return 0
}

func main() {
	flag.Parse()
	// move code into stage1() helper so defered fns get run
	os.Exit(stage1())
}

func launchMetadataSvc() error {
	fmt.Println("Launching metadatasvc: ", metadataSvc)

	// use socket activation protocol to avoid race-condition of
	// service becoming ready
	// TODO(eyakubovich): remove hard-coded port
	l, err := net.ListenTCP("tcp4", &net.TCPAddr{Port: metadata.MetadataSvcPrvPort})
	if err != nil {
		if err.(*net.OpError).Err.(*os.SyscallError).Err == syscall.EADDRINUSE {
			// assume metadatasvc is already running
			return nil
		}
		return err
	}

	defer l.Close()

	lf, err := l.File()
	if err != nil {
		return err
	}

	// parse metadataSvc into exe and args
	args := strings.Split(metadataSvc, " ")
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Env = append(os.Environ(), "LISTEN_FDS=1")
	cmd.ExtraFiles = []*os.File{lf}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Start()
}
