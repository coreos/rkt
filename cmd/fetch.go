package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/coreos-inc/rkt/store"
)

const (
	imgDir = "images"
)

var (
	cmdFetch = &Command{
		Name:    "fetch",
		Summary: "Fetch image(s) and store them in the local cache",
		Usage:   "IMAGE_URL...",
		Run:     runFetch,
	}
)

func fetchURL(img string, ds *store.Store) (string, error) {
	rem := store.NewRemote(img, []string{})
	err := ds.Get(rem)
	if err != nil && rem.File == "" {
		rem, err = rem.Download(*ds)
		if err != nil {
			return "", fmt.Errorf("downloading: %v\n", err)
		}
	}
	return rem.File, nil
}

func runFetch(args []string) (exit int) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "fetch: Must provide at least one image\n")
		return 1
	}
	root := filepath.Join(globalFlags.Dir, imgDir)
	if err := os.MkdirAll(root, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "fetch: error creating image directory: %v", err)
		return 1
	}

	ds := store.NewStore(globalFlags.Dir)

	for _, img := range args {
		hash, err := fetchURL(img, ds)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v", err)
			return 1
		}
		fmt.Println(hash)
	}

	return
}
