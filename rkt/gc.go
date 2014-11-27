package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/coreos/rocket/pkg/lock"
)

var (
	cmdGC = &Command{
		Name:    "gc",
		Summary: "Garbage-collect rkt containers no longer in use",
		Usage:   "",
		Run:     runGC,
	}
)

func init() {
	commands = append(commands, cmdGC)
}

func runGC(args []string) (exit int) {
	cs, err := getContainers(globalFlags.Dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gc: %v\n", err)
		return 1
	}
	for _, c := range cs {
		fmt.Printf("Garbage collecting container %s\n", c)
		c := filepath.Join(containersDir(globalFlags.Dir), c)
		l, err := lock.NewLock(c)
		if err != nil {
			fmt.Println(err)
			continue
		}
		err = l.ExclusiveLock()
		if err != nil {
			fmt.Println(err)
		}
	}
	fmt.Println("sleeping 5s")
	time.Sleep(5 * time.Second)
	fmt.Println("execing ex")
	syscall.Exec("/home/core/ex", []string{"/home/core/ex"}, nil)
	return
}

// getContainers returns a slice representing the containers in the given rocket directory
func getContainers(rktDir string) ([]string, error) {
	cdir := containersDir(globalFlags.Dir)
	ls, err := ioutil.ReadDir(cdir)
	if err != nil {
		return nil, fmt.Errorf("cannot read containers directory: %v", err)
	}
	var cs []string
	for _, dir := range ls {
		if !dir.IsDir() {
			fmt.Fprintf(os.Stderr, "unrecognized file: %q, ignoring", dir)
			continue
		}
		cs = append(cs, dir.Name())
	}
	return cs, nil
}
