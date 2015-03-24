// Copyright 2015 CoreOS, Inc.
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
	"flag"
	"io/ioutil"
	"log"
	"os"

	"github.com/coreos/rocket/Godeps/_workspace/src/github.com/appc/spec/schema/types"
	"github.com/coreos/rocket/cas"
	"github.com/coreos/rocket/stage0"
)

var (
	cmdPrepare = &Command{
		Name:    "prepare",
		Summary: "Prepare to run image(s) in an application container in rocket",
		Usage:   "[--volume name,kind=host,...] [--quiet] IMAGE [-- image-args...[---]]...",
		Description: `Image should be a string referencing an image; either a hash, local file on disk, or URL.
They will be checked in that order and the first match will be used.

An "--" may be used to inhibit rkt prepare's parsing of subsequent arguments,
which will instead be appended to the preceding image app's exec arguments.
End the image arguments with a lone "---" to resume argument parsing.`,
		Run:   runPrepare,
		Flags: &prepareFlags,
	}
	prepareFlags flag.FlagSet
	flagQuiet    bool
)

func init() {
	commands = append(commands, cmdPrepare)
	prepareFlags.StringVar(&flagStage1Image, "stage1-image", defaultStage1Image, `image to use as stage1. Local paths and http/https URLs are supported. If empty, Rocket will look for a file called "stage1.aci" in the same directory as rkt itself`)
	prepareFlags.Var(&flagVolumes, "volume", "volumes to mount into the shared container environment")
	prepareFlags.BoolVar(&flagQuiet, "quiet", false, "suppress superfluous output on stdout, print only the UUID on success")
	prepareFlags.BoolVar(&flagInheritEnv, "inherit-env", false, "inherit all environment variables not set by apps")
	prepareFlags.Var(&flagExplicitEnv, "set-env", "an environment variable to set for apps in the form name=value")
}

func runPrepare(args []string) (exit int) {
	var err error
	origStdout := os.Stdout
	if flagQuiet {
		if os.Stdout, err = os.Open("/dev/null"); err != nil {
			stderr("prepare: unable to open /dev/null")
			return 1
		}
	}

	appArgs, images, err := parseAppArgs(args, &prepareFlags)
	if err != nil {
		stderr("prepare: error parsing app image arguments")
		return 1
	}

	if len(images) < 1 {
		stderr("prepare: Must provide at least one image")
		return 1
	}
	if globalFlags.Dir == "" {
		log.Printf("dir unset - using temporary directory")
		globalFlags.Dir, err = ioutil.TempDir("", "rkt")
		if err != nil {
			stderr("prepare: error creating temporary directory: %v", err)
			return 1
		}
	}

	ds, err := cas.NewStore(globalFlags.Dir)
	if err != nil {
		stderr("prepare: cannot open store: %v", err)
		return 1
	}
	ks := getKeystore()

	s1img, err := findImage(flagStage1Image, ds, ks, false)
	if err != nil {
		stderr("prepare: finding stage1 image %q: %v", flagStage1Image, err)
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
		stderr("prepare: error creating new container: %v", err)
		return 1
	}

	pcfg := stage0.PrepareConfig{
		CommonConfig: stage0.CommonConfig{
			Store:       ds,
			Debug:       globalFlags.Debug,
			Stage1Image: *s1img,
			UUID:        c.uuid,
			Images:      imgs,
		},
		ExecAppends: appArgs,
		InheritEnv:  flagInheritEnv,
		ExplicitEnv: flagExplicitEnv.Strings(),
		Volumes:     []types.Volume(flagVolumes),
	}

	if err = stage0.Prepare(pcfg, c.path(), c.uuid); err != nil {
		stderr("prepare: error setting up stage0: %v", err)
		return 1
	}

	if err := c.xToPrepared(); err != nil {
		stderr("prepare: error transitioning to prepared: %v", err)
		return 1
	}

	os.Stdout = origStdout // restore output in case of --quiet
	stdout("%s", c.uuid.String())

	return 0
}
