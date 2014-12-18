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

func initialSetupAndArgs(debug bool) (*Container, []string) {
	root := "."
	c, err := LoadContainer(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load container: %v\n", err)
		os.Exit(1)
	}

	mirrorLocalZoneInfo(c.Root)

	args := []string{
		filepath.Join(path.Stage1RootfsPath(c.Root), interpBin),
		filepath.Join(path.Stage1RootfsPath(c.Root), nspawnBin),
		"--register", "false", // We cannot assume the host system is running systemd
	}

	if !debug {
		args = append(args, "--quiet") // silence most nspawn output (log_warning is currently not covered by this)
	}

	return c, args
}

func executeNspawn(root string, args []string) {
	env := os.Environ()
	env = append(env, "LD_PRELOAD="+filepath.Join(path.Stage1RootfsPath(root), "fakesdboot.so"))
	env = append(env, "LD_LIBRARY_PATH="+filepath.Join(path.Stage1RootfsPath(root), "usr/lib"))

	if err := syscall.Exec(args[0], args, env); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to execute nspawn: %v\n", err)
		os.Exit(5)
	}
}

func run(debug bool, opt_args []string) {
	c, args := initialSetupAndArgs(debug)

	if err := c.ContainerToSystemd(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to configure systemd: %v\n", err)
		os.Exit(2)
	}

	// Launch systemd in the container
	args = append(args, "--boot")
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

	executeNspawn(c.Root, args)
}

func enter(debug bool, opt_args []string) {
	c, args := initialSetupAndArgs(debug)

	args = append(args, c.ContainerToNspawnForEnterArgs()...)
	// Binary to execute
	args = append(args, "--")
	args = append(args, opt_args...)

	executeNspawn(c.Root, args)
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Expected at least action parameter (run or enter)\n")
		os.Exit(1)
	}
	action := os.Args[1];
	no_interp_index := 2
	debug := false
	if len(os.Args) > 2 && os.Args[2] == "debug" {
		debug = true
		no_interp_index = 3
	}

	if len(os.Args) > no_interp_index && os.Args[no_interp_index] != "--" {
		fmt.Fprintf(os.Stderr, "Expected '--' followed by optional parameters\n")
		os.Exit(1)
	}
	opt_args := []string{}
	if len(os.Args) > no_interp_index + 1 {
		opt_args = os.Args[no_interp_index + 1:]
	}
	switch action {
	case "run":
		run(debug, opt_args)
	case "enter":
		enter(debug, opt_args)
	default:
		fmt.Fprintf(os.Stderr, "Unknown action: %s\n", action)
	}
}
