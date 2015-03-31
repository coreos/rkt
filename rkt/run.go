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
	"github.com/coreos/rocket/common"
	"github.com/coreos/rocket/pkg/keystore"
	"github.com/coreos/rocket/stage0"
)

var (
	defaultStage1Image string // either set by linker, or guessed in init()

	flagStage1Image          string
	flagVolumes              volumeList
	flagPrivateNet           bool
	flagSpawnMetadataService bool
	flagInheritEnv           bool
	flagExplicitEnv          envMap
	flagInteractive          bool
	flagNoOverlay            bool
	cmdRun                   = &Command{
		Name:    "run",
		Summary: "Run image(s) in an application container in rocket",
		Usage:   "[--volume name,kind=host,...] IMAGE [-- image-args...[---]]...",
		Description: `IMAGE should be a string referencing an image; either a hash, local file on disk, or URL.
They will be checked in that order and the first match will be used.

An "--" may be used to inhibit rkt run's parsing of subsequent arguments,
which will instead be appended to the preceding image app's exec arguments.
End the image arguments with a lone "---" to resume argument parsing.`,
		Run: runRun,
	}
)

func init() {
	commands = append(commands, cmdRun)

	// if not set by linker, try discover the directory rkt is running
	// from, and assume the default stage1.aci is stored alongside it.
	if defaultStage1Image == "" {
		if exePath, err := os.Readlink("/proc/self/exe"); err == nil {
			defaultStage1Image = filepath.Join(filepath.Dir(exePath), "stage1.aci")
		}
	}

	cmdRun.Flags.StringVar(&flagStage1Image, "stage1-image", defaultStage1Image, `image to use as stage1. Local paths and http/https URLs are supported. If empty, Rocket will look for a file called "stage1.aci" in the same directory as rkt itself`)
	cmdRun.Flags.Var(&flagVolumes, "volume", "volumes to mount into the shared container environment")
	cmdRun.Flags.BoolVar(&flagPrivateNet, "private-net", false, "give container a private network")
	cmdRun.Flags.BoolVar(&flagSpawnMetadataService, "spawn-metadata-svc", false, "launch metadata svc if not running")
	cmdRun.Flags.BoolVar(&flagInheritEnv, "inherit-env", false, "inherit all environment variables not set by apps")
	cmdRun.Flags.BoolVar(&flagNoOverlay, "no-overlay", false, "disable overlay filesystem")
	cmdRun.Flags.Var(&flagExplicitEnv, "set-env", "an environment variable to set for apps in the form name=value")
	cmdRun.Flags.BoolVar(&flagInteractive, "interactive", false, "the container is interactive")
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

	// try fetching the image, potentially remotely
	key, err := fetchImage(img, "" /* TODO(vc): wire up --signature */, ds, ks, discover)
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

// parseAppArgs looks through the remaining arguments for support of per-app argument lists delimited with "--" and "---"
func parseAppArgs(args []string) ([][]string, []string, error) {
	// valid args here may either be:
	// not-"--"; an image specifier
	// "--"; image arguments begin
	// "---"; conclude image arguments
	appArgs := make([][]string, 0)
	images := make([]string, 0)
	inAppArgs := false
	for _, a := range args {
		if inAppArgs {
			switch a {
			case "---":
				// conclude this app's args
				inAppArgs = false
			default:
				// keep appending to this app's args
				appArgs[len(appArgs)-1] = append(appArgs[len(appArgs)-1], a)
			}
		} else {
			switch a {
			case "--":
				// begin app's args, TODO(vc): this could be made more strict/police if deemed necessary
				inAppArgs = true
			case "---":
				// ignore triple dashes since they aren't images
			default:
				// this is something else, append it to images
				// TODO(vc): for now these basically have to be images, but it should be possible to reenter cmdRun.flags.Parse()
				images = append(images, a)
				appArgs = append(appArgs, make([]string, 0))
			}
		}
	}

	return appArgs, images, nil
}

func runRun(args []string) (exit int) {
	if flagInteractive && len(args) > 1 {
		stderr("run: interactive option only supports one image")
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

	appArgs, images, err := parseAppArgs(args)
	if err != nil {
		stderr("run: error parsing app image arguments")
		return 1
	}

	if len(images) < 1 {
		stderr("run: Must provide at least one image")
		return 1
	}

	ds, err := cas.NewStore(globalFlags.Dir)
	if err != nil {
		stderr("run: cannot open store: %v", err)
		return 1
	}
	ks := getKeystore()

	s1img, err := findImage(flagStage1Image, ds, nil, false)
	if err != nil {
		stderr("Error finding stage1 image %q: %v", flagStage1Image, err)
		return 1
	}

	imgs, err := findImages(images, ds, ks)
	if err != nil {
		stderr("%v", err)
		return 1
	}

	if len(imgs) != len(appArgs) {
		stderr("Unexpected mismatch of app args and app images")
		return 1
	}

	c, err := newContainer()
	if err != nil {
		stderr("Error creating new container: %v", err)
		return 1
	}

	cfg := stage0.CommonConfig{
		Store:       ds,
		Stage1Image: *s1img,
		UUID:        c.uuid,
		Images:      imgs,
		Debug:       globalFlags.Debug,
	}

	pcfg := stage0.PrepareConfig{
		CommonConfig: cfg,
		ExecAppends:  appArgs,
		Volumes:      []types.Volume(flagVolumes),
		InheritEnv:   flagInheritEnv,
		ExplicitEnv:  flagExplicitEnv.Strings(),
		UseOverlay:   !flagNoOverlay && common.SupportsOverlay(),
	}
	err = stage0.Prepare(pcfg, c.path(), c.uuid)
	if err != nil {
		stderr("run: error setting up stage0: %v", err)
		return 1
	}

	// get the lock fd for run
	lfd, err := c.Fd()
	if err != nil {
		stderr("Error getting container lock fd: %v", err)
		return 1
	}

	// skip prepared by jumping directly to run, we own this container
	if err := c.xToRun(); err != nil {
		stderr("run: unable to transition to run: %v", err)
		return 1
	}

	rcfg := stage0.RunConfig{
		CommonConfig:         cfg,
		PrivateNet:           flagPrivateNet,
		SpawnMetadataService: flagSpawnMetadataService,
		LockFd:               lfd,
		Interactive:          flagInteractive,
	}
	stage0.Run(rcfg, c.path()) // execs, never returns

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

// envMap implements the flag.Value interface to contain a set of name=value mappings
type envMap struct {
	mapping map[string]string
}

func (e *envMap) Set(s string) error {
	if e.mapping == nil {
		e.mapping = make(map[string]string)
	}
	pair := strings.SplitN(s, "=", 2)
	if len(pair) != 2 {
		return fmt.Errorf("environment variable must be specified as name=value")
	}
	if _, exists := e.mapping[pair[0]]; exists {
		return fmt.Errorf("environment variable %q already set", pair[0])
	}
	e.mapping[pair[0]] = pair[1]
	return nil
}

func (e *envMap) String() string {
	return strings.Join(e.Strings(), "\n")
}

func (e *envMap) Strings() []string {
	var env []string
	for n, v := range e.mapping {
		env = append(env, n+"="+v)
	}
	return env
}
