//+build linux

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/appc/spec/schema/types"
	rktpath "github.com/coreos/rocket/path"
	"github.com/coreos/rocket/pkg/lock"
	"github.com/coreos/rocket/stage0"
)

const (
	defaultCmd = "/bin/bash"
)

var (
	cmdEnter = &Command{
		Name:    "enter",
		Summary: "Enter the namespaces of a rkt job",
		Usage:   "--uuid UUID --imageid IMAGEID [CMD [ARGS ...]]",
		Run:     runEnter,
	}
	flagContainerUUID types.UUID
	flagAppImageID    types.Hash
)

func init() {
	commands = append(commands, cmdEnter)
	cmdEnter.Flags.Var(&flagContainerUUID, "uuid", "uuid of container to enter")
	cmdEnter.Flags.Var(&flagAppImageID, "imageid", "imageid of the app to enter within the specified container")
}

func runEnter(args []string) (exit int) {
	needargs := bool(false)

	if flagContainerUUID.Empty() {
		fmt.Fprintf(os.Stderr, "--uuid required\n")
		needargs = true
	}

	if flagAppImageID.Empty() {
		fmt.Fprintf(os.Stderr, "--imageid required\n")
		needargs = true
	}

	if needargs {
		// TODO(vc): trigger usage print?
		return 1
	}

	cp := filepath.Join(containersDir(), flagContainerUUID.String())
	l, err := lock.NewLock(cp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to get container lock: %v\n", err)
		return 1
	}
	defer l.Close()
	if err = l.TrySharedLock(); err == nil {
		fmt.Fprintf(os.Stderr, "Container %q inactive\n", flagContainerUUID.String())
		return 1
	}
	if err != lock.ErrLocked {
		fmt.Fprintf(os.Stderr, "Lock error: %v\n", err)
		return 1
	}

	_, err = os.Stat(filepath.Join(rktpath.AppRootfsPath(cp, flagAppImageID)))
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to access imageid rootfs: %v\n", err)
		return 1
	}

	if len(args) < 1 {
		fmt.Printf("No command specified, assuming %q\n", defaultCmd)

		_, err := exec.LookPath(filepath.Join(rktpath.AppRootfsPath(cp, flagAppImageID), defaultCmd))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Default command %q unusable in image, giving up: %v\n", defaultCmd, err)
			return 1
		}
		args = append(args, defaultCmd)
	}

	err = stage0.Enter(filepath.Join(containersDir(), flagContainerUUID.String()), flagAppImageID.String(), args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Enter failed: %v\n", err)
		return 1
	}
	// not reached when stage0.Enter execs /enter
	return 0
}
