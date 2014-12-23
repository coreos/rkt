//+build linux

package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/appc/spec/schema/types"
	"github.com/coreos/rocket/pkg/lock"
)

var (
	cmdStatus = &Command{
		Name:    "status",
		Summary: "Check the status of a rkt job",
		Usage:   "--uuid UUID [--wait]",
		Run:     runStatus,
	}
	flagContainerUUID types.UUID
	flagWait          bool
)

func init() {
	commands = append(commands, cmdStatus)
	cmdStatus.Flags.Var(&flagContainerUUID, "uuid", "uuid of container to query the status of")
	cmdStatus.Flags.BoolVar(&flagWait, "wait", false, "toggle waiting for the container to exit if running")
}

func runStatus(args []string) (exit int) {
	isGarbage := false

	if flagContainerUUID.Empty() {
		fmt.Fprintf(os.Stderr, "--uuid required\n")
		return 1
	}

	cp := filepath.Join(containersDir(), flagContainerUUID.String())
	l, err := lock.NewLock(cp)
	if err != nil {
		cp = filepath.Join(garbageDir(), flagContainerUUID.String())
		l, err = lock.NewLock(cp)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to open lock for container %q: %v\n", flagContainerUUID.String(), err)
			return 2
		}
		isGarbage = true
	}
	defer l.Close()

	exited := true
	if flagWait && !isGarbage {
		err = l.SharedLock()
	} else {
		err = l.TrySharedLock()
		if err == lock.ErrLocked {
			if isGarbage {
				// container is exited and being deleted, we can't reliably query its status, it's effectively gone.
				fmt.Fprintf(os.Stderr, "Unable to query status: %q is being removed\n", flagContainerUUID.String())
				return 3
			}
			exited = false
			err = nil
		}
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Lock error: %v\n", err)
		return 4
	}

	if err := printStatus(cp, exited); err != nil {
		fmt.Fprintf(os.Stderr, "Unable to print status: %v\n", err)
		return 5
	}

	return 0
}

// printStatus prints the container's pid and per-app status codes
func printStatus(cpath string, exited bool) error {
	pid, err := getIntFromFile(filepath.Join(cpath, "pid"))
	if err != nil {
		return err
	}

	stats, err := getStatuses(cpath)
	if err != nil {
		return err
	}

	fmt.Printf("pid=%d\nexited=%v\n", pid, exited)
	for app, stat := range stats {
		fmt.Printf("%s=%d\n", app, stat)
	}
	return nil
}

// getStatuses returns a map of imageId:status codes for the given container
func getStatuses(cpath string) (map[string]int, error) {
	sdir := filepath.Join(cpath, "stage1/rkt/status")
	ls, err := ioutil.ReadDir(sdir)
	if err != nil {
		return nil, fmt.Errorf("unable to read status directory: %v", err)
	}

	stats := make(map[string]int)
	for _, ent := range ls {
		if ent.IsDir() {
			continue
		}
		s, err := getIntFromFile(filepath.Join(sdir, ent.Name()))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to get status of app %q: %v\n", ent.Name(), err)
			continue
		}
		stats[ent.Name()] = s
	}

	return stats, err
}

// getIntFromFile reads an integer string from the named file
func getIntFromFile(path string) (i int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	buf, err := ioutil.ReadAll(f)
	if err != nil {
		return
	}

	_, err = fmt.Sscanf(string(buf), "%d", &i)
	if err != nil {
		return
	}

	return
}
