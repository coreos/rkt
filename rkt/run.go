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

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/appc/spec/schema/types"
	"github.com/coreos/rocket/cas"
	"github.com/coreos/rocket/stage0"
)

var (
	flagStage1Init       string
	flagStage1Rootfs     string
	flagVolumes          volumeMap
	flagSpawnMetadataSvc bool
	cmdRun               = &Command{
		Name:    "run",
		Summary: "Run image(s) in an application container in rocket",
		Usage:   "[--volume LABEL:SOURCE] IMAGE...",
		Description: `IMAGE should be a string referencing an image; either a hash, local file on disk, or URL.
They will be checked in that order and the first match will be used.`,
		Run: runRun,
	}
)

func init() {
	commands = append(commands, cmdRun)
	cmdRun.Flags.StringVar(&flagStage1Init, "stage1-init", "", "path to stage1 binary override")
	cmdRun.Flags.StringVar(&flagStage1Rootfs, "stage1-rootfs", "", "path to stage1 rootfs tarball override")
	cmdRun.Flags.Var(&flagVolumes, "volume", "volumes to mount into the shared container environment")
	cmdRun.Flags.BoolVar(&flagSpawnMetadataSvc, "spawn-metadata-svc", false, "launch metadata svc if not running")
	flagVolumes = volumeMap{}
}

// findImages will recognize a ACI hash and use that, import a local file, use
// discovery or download an ACI directly.
func findImages(args []string, ds *cas.Store) (out []types.Hash, err error) {
	out = make([]types.Hash, len(args))
	for i, img := range args {
		// check if it is a valid hash, if so let it pass through
		h, err := types.NewHash(img)
		if err == nil {
			fullKey, err := ds.ResolveKey(img)
			if err != nil {
				return nil, fmt.Errorf("could not resolve key: %v", err)
			}
			h, err = types.NewHash(fullKey)
			if err != nil {
				// should never happen
				panic(err)
			}
			out[i] = *h
			continue
		}

		// import the local file if it exists
		file, err := os.Open(img)
		if err == nil {
			key, err := ds.WriteACI(file)
			file.Close()
			if err != nil {
				return nil, fmt.Errorf("%s: %v", img, err)
			}
			h, err := types.NewHash(key)
			if err != nil {
				// should never happen
				panic(err)
			}
			out[i] = *h
			continue
		}

		key, err := fetchImage(img, ds)
		if err != nil {
			return nil, err
		}
		h, err = types.NewHash(key)
		if err != nil {
			// should never happen
			panic(err)
		}
		out[i] = *h
	}

	return out, nil
}

func runRun(args []string) (exit int) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "run: Must provide at least one image\n")
		return 1
	}
	if globalFlags.Dir == "" {
		log.Printf("dir unset - using temporary directory")
		var err error
		globalFlags.Dir, err = ioutil.TempDir("", "rkt")
		if err != nil {
			fmt.Fprintf(os.Stderr, "error creating temporary directory: %v\n", err)
			return 1
		}
	}

	ds := cas.NewStore(globalFlags.Dir)
	imgs, err := findImages(args, ds)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}

	cfg := stage0.Config{
		Store:            ds,
		ContainersDir:    containersDir(),
		Debug:            globalFlags.Debug,
		Stage1Init:       flagStage1Init,
		Stage1Rootfs:     flagStage1Rootfs,
		Images:           imgs,
		Volumes:          flagVolumes,
		SpawnMetadataSvc: flagSpawnMetadataSvc,
	}
	cdir, err := stage0.Setup(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "run: error setting up stage0: %v\n", err)
		return 1
	}
	stage0.Run(cfg, cdir) // execs, never returns
	return 1
}

// volumeMap implements the flag.Value interface to contain a set of mappings
// from mount label --> mount path
type volumeMap map[string]string

func (vm *volumeMap) Set(s string) error {
	elems := strings.Split(s, ":")
	if len(elems) != 2 {
		return errors.New("volume must be of form key:path")
	}
	key := elems[0]
	if _, ok := (*vm)[key]; ok {
		return fmt.Errorf("got multiple flags for volume %q", key)
	}
	(*vm)[key] = elems[1]
	return nil
}

func (vm *volumeMap) String() string {
	var ss []string
	for k, v := range *vm {
		ss = append(ss, fmt.Sprintf("%s:%s", k, v))
	}
	return strings.Join(ss, ",")
}
