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
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/coreos/rocket/Godeps/_workspace/src/github.com/appc/spec/schema/types"
	"github.com/coreos/rocket/cas"
	"github.com/coreos/rocket/pkg/keystore"
	"github.com/coreos/rocket/stage0"
)

var (
	flagStage1Image      string
	flagVolumes          volumeList
	flagPrivateNet       bool
	flagSpawnMetadataSvc bool
	cmdRun               = &Command{
		Name:    "run",
		Summary: "Run image(s) in an application container in rocket",
		Usage:   "[--volume name,type=host...] IMAGE...",
		Description: `IMAGE should be a string referencing an image; either a hash, local file on disk, or URL.
They will be checked in that order and the first match will be used.`,
		Run: runRun,
	}
)

func init() {
	commands = append(commands, cmdRun)

	// try discover the directory rkt is running from, assume the default stage1.aci is stored alongside it.
	defaultStage1Image := ""
	exePath, err := os.Readlink("/proc/self/exe")
	if err == nil {
		defaultStage1Image = filepath.Join(filepath.Dir(exePath), "stage1.aci")
	}

	cmdRun.Flags.StringVar(&flagStage1Image, "stage1-image", defaultStage1Image, "image to use as stage1, local paths and http/https urls are supported")
	cmdRun.Flags.Var(&flagVolumes, "volume", "volumes to mount into the shared container environment")
	cmdRun.Flags.BoolVar(&flagPrivateNet, "private-net", false, "give container a private network")
	cmdRun.Flags.BoolVar(&flagSpawnMetadataSvc, "spawn-metadata-svc", false, "launch metadata svc if not running")
	flagVolumes = volumeList{}
}

// findImages uses findImage to attain a list of image hashes using discovery if necessary
func findImages(args []string, ds *cas.Store, ks *keystore.Keystore) (out []types.Hash, err error) {
	out = make([]types.Hash, len(args))
	for i, img := range args {
		h, err := findImage(img, ds, ks, true)
		if err != nil {
			return nil, err
		}
		out[i] = *h
	}

	return out, nil
}

// findImage will recognize a ACI hash and use that, import a local file, use
// discovery or download an ACI directly.
func findImage(img string, ds *cas.Store, ks *keystore.Keystore, discover bool) (*types.Hash, error) {
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
		return h, nil
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
		return h, nil
	}

	// try fetching remotely
	key, err := fetchImage(img, ds, ks, discover)
	if err != nil {
		return nil, err
	}
	h, err = types.NewHash(key)
	if err != nil {
		// should never happen
		panic(err)
	}

	return h, nil
}

func runRun(args []string) (exit int) {
	if len(args) < 1 {
		stderr("run: Must provide at least one image")
		return 1
	}
	if globalFlags.Dir == "" {
		log.Printf("dir unset - using temporary directory")
		var err error
		globalFlags.Dir, err = ioutil.TempDir("", "rkt")
		if err != nil {
			stderr("error creating temporary directory: %v", err)
			return 1
		}
	}

	ds := cas.NewStore(globalFlags.Dir)
	ks := getKeystore()

	s1img, err := findImage(flagStage1Image, ds, ks, false)
	if err != nil {
		stderr("Error finding stage1 image %q: %v", flagStage1Image, err)
		return 1
	}

	imgs, err := findImages(args, ds, ks)
	if err != nil {
		stderr("%v", err)
		return 1
	}

	cfg := stage0.Config{
		Store:            ds,
		ContainersDir:    containersDir(),
		Debug:            globalFlags.Debug,
		Stage1Image:      *s1img,
		Images:           imgs,
		Volumes:          []types.Volume(flagVolumes),
		PrivateNet:       flagPrivateNet,
		SpawnMetadataSvc: flagSpawnMetadataSvc,
	}
	cdir, err := stage0.Setup(cfg)
	if err != nil {
		stderr("run: error setting up stage0: %v", err)
		return 1
	}
	stage0.Run(cfg, cdir) // execs, never returns
	return 1
}

// volumeList implements the flag.Value interface to contain a set of mappings
// from mount label --> mount path
type volumeList []types.Volume

func (vl *volumeList) Set(s string) error {
	vol, err := types.VolumeFromString(s)
	if err != nil {
		return err
	}

	*vl = append(*vl, *vol)
	return nil
}

func (vl *volumeList) String() string {
	var vs []string
	for _, v := range []types.Volume(*vl) {
		vs = append(vs, v.String())
	}
	return strings.Join(vs, " ")
}
