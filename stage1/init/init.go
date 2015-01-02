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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

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
	const localtime = "/etc/localtime"

	// Make sure src exists and neither are special files
	issymlink := false
	if fi, err := os.Lstat(localtime); err != nil {
		return
	} else {
		mode := fi.Mode()
		switch {
		case mode.IsRegular():
			break
		case mode&os.ModeType == os.ModeSymlink:
			issymlink = true
		default:
			// "Unsupported filemode for /etc/localtime: %q\n", fi.Mode()
			return
		}
	}

	var srcp, destp = localtime,
		filepath.Join(path.Stage1RootfsPath(root), localtime)

	if issymlink {
		// for symlink, need to create the symlink first
		zif, err := os.Readlink(localtime)
		if err != nil {
			return
		}

		if err = os.MkdirAll(filepath.Dir(destp), 0755); err != nil {
			return
		}
		if err = os.Symlink(zif, destp); err != nil {
			return
		}

		if filepath.IsAbs(zif) {
			srcp = zif
		} else {
			srcp = filepath.Join(filepath.Dir(srcp), zif)
		}

		destp = filepath.Join(path.Stage1RootfsPath(root), srcp)
	}

	src, err := os.Open(srcp)
	if err != nil {
		return
	}
	defer src.Close()

	if err = os.MkdirAll(filepath.Dir(destp), 0755); err != nil {
		return
	}

	dest, err := os.OpenFile(destp, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer dest.Close()

	io.Copy(dest, src)
}

func main() {
	root := "."
	debug := len(os.Args) > 1 && os.Args[1] == "debug"

	c, err := LoadContainer(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load container: %v\n", err)
		os.Exit(1)
	}

	mirrorLocalZoneInfo(c.Root)

	if err = c.ContainerToSystemd(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to configure systemd: %v\n", err)
		os.Exit(2)
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
		os.Exit(4)
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

	if err := syscall.Exec(args[0], args, env); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to execute nspawn: %v\n", err)
		os.Exit(5)
	}
}
