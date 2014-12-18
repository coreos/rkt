//+build linux

package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/appc/spec/schema/types"
	"github.com/coreos/rocket/cas"
)

// findImage will recognize a ACI hash and use that, import a local file, use
// discovery or download an ACI directly.
func findImage(img string, ds *cas.Store) (out types.Hash, err error) {
	// check if it is a valid hash, if so let it pass through
	h, err := types.NewHash(img)
	if err == nil {
		out = *h
		return out, nil
	}

	// import the local file if it exists
	file, err := os.Open(img)
	if err == nil {
		tmp := types.NewHashSHA256([]byte(img)).String()
		key, err := ds.WriteACI(tmp, file)
		file.Close()
		if err != nil {
			return out, fmt.Errorf("%s: %v", img, err)
		}
		h, err := types.NewHash(key)
		if err != nil {
			// should never happen
			panic(err)
		}
		out = *h
		return out, nil
	}

	key, err := fetchImage(img, ds)
	if err != nil {
		return out, err
	}
	h, err = types.NewHash(key)
	if err != nil {
		// should never happen
		panic(err)
	}
	out = *h

	return out, nil
}

func getDir() (string, error) {
	gdir := globalFlags.Dir
	if gdir == "" {
		log.Printf("dir unset - using temporary directory")
		var err error
		gdir, err = ioutil.TempDir("", "rkt")
		if err != nil {
			fmt.Fprintf(os.Stderr, "error creating temporary directory: %v\n", err)
			return "", err
		}
	}
	return gdir, nil
}
