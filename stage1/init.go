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

	env = append(env, "LISTEN_FDS=1")
	pid := fmt.Sprintf("%d", os.Getpid())
	env = append(env, "LISTEN_PID="+pid)
	fmt.Println(env)

	if err := syscall.Exec(args[0], args, env); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to execute nspawn: %v\n", err)
		os.Exit(5)
	}
}
