//+build linux

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/appc/spec/schema/types"
	"github.com/coreos/rocket/cas"
	"github.com/coreos/rocket/stage0"
)

var (
	cmdEnter         = &Command{
		Name:    "enter",
		Summary: "Enter image(s) in an application container in rocket",
		Usage:   "IMAGE [APP [OPTIONS]]",
		Description: `IMAGE should be a string referencing an image; either a hash, local file on disk, or URL.
They will be checked in that order and the first match will be used.
APP should be a binary to be run in container (e.g. /bin/bash). If omitted
then root shell is ran`,
		Run: runEnter,
	}
)

func init() {
	commands = append(commands, cmdEnter)
}

func runEnter(args []string) (exit int) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "enter: Must provide an image\n")
		return 1
	}
	cmds := []string{}
	if len(args) > 1 {
		cmds = args[1:]
	}
	gdir, err := getDir()
	if err != nil {
		return 1
	}

	ds := cas.NewStore(gdir)
	img, err := findImage(args[0], ds)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}

	// TODO(jonboulle): use rkt/path
	cdir := filepath.Join(gdir, "containers")
	cfg := stage0.Config{
		Store:         ds,
		ContainersDir: cdir,
		Debug:         globalFlags.Debug,
		Images:        []types.Hash{img},
	}
	cdir, err = stage0.Setup(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "enter: error setting up stage0: %v\n", err)
		return 1
	}
	stage0.Enter(cdir, cmds, cfg.Debug) // execs, never returns
	return 1
}
