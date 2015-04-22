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

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/schema/types"
	"github.com/coreos/rkt/common"
	"github.com/coreos/rkt/stage0"
	"github.com/coreos/rkt/store"
)

var (
	cmdPrepare = &Command{
		Name:    "prepare",
		Summary: "Prepare to run image(s) in a pod in rkt",
		Usage:   "[--volume name,kind=host,...] [--quiet] IMAGE [-- image-args...[---]]...",
		Description: `Image should be a string referencing an image; either a hash, local file on disk, or URL.
They will be checked in that order and the first match will be used.

An "--" may be used to inhibit rkt prepare's parsing of subsequent arguments,
which will instead be appended to the preceding image app's exec arguments.
End the image arguments with a lone "---" to resume argument parsing.`,
		Run:                  runPrepare,
		Flags:                &prepareFlags,
		WantsFlagsTerminator: true,
	}
	prepareFlags flag.FlagSet
	flagQuiet    bool
)

func init() {
	commands = append(commands, cmdPrepare)
	prepareFlags.StringVar(&flagStage1Image, "stage1-image", defaultStage1Image, `image to use as stage1. Local paths and http/https URLs are supported. If empty, rkt will look for a file called "stage1.aci" in the same directory as rkt itself`)
	prepareFlags.Var(&flagPorts, "port", "ports to expose on the host (requires --private-net)")
	prepareFlags.BoolVar(&flagQuiet, "quiet", false, "suppress superfluous output on stdout, print only the UUID on success")
	prepareFlags.BoolVar(&flagInheritEnv, "inherit-env", false, "inherit all environment variables not set by apps")
	prepareFlags.BoolVar(&flagNoOverlay, "no-overlay", false, "disable overlay filesystem")
	prepareFlags.Var(&flagExplicitEnv, "set-env", "an environment variable to set for apps in the form name=value")
	prepareFlags.BoolVar(&flagLocal, "local", false, "use only local images (do not discover or download from remote URLs)")
	prepareFlags.Var((*appsVolume)(&rktApps), "volume", "volumes to make available in the pod")
	prepareFlags.Var((*appMount)(&rktApps), "mount", "mount point binding a volume to a path within an app")
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

	if err = parseApps(&rktApps, args, &prepareFlags, true); err != nil {
		stderr("prepare: error parsing app image arguments: %v", err)
		return 1
	}

	if rktApps.Count() < 1 {
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

	s, err := store.NewStore(globalFlags.Dir)
	if err != nil {
		stderr("prepare: cannot open store: %v", err)
		return 1
	}

	config, err := getConfig()
	if err != nil {
		stderr("prepare: cannot get configuration: %v", err)
		return 1
	}
	fn := &finder{
		imageActionData: imageActionData{
			s:                  s,
			headers:            config.AuthPerHost,
			dockerAuth:         config.DockerCredentialsPerRegistry,
			insecureSkipVerify: globalFlags.InsecureSkipVerify,
			debug:              globalFlags.Debug,
		},
		local: flagLocal,
	}
	s1img, err := fn.findImage(flagStage1Image, "", false)
	if err != nil {
		stderr("prepare: finding stage1 image %q: %v", flagStage1Image, err)
		return 1
	}

	fn.ks = getKeystore()
	if err := fn.findImages(&rktApps); err != nil {
		stderr("%v", err)
		return 1
	}

	p, err := newPod()
	if err != nil {
		stderr("prepare: error creating new pod: %v", err)
		return 1
	}

	pcfg := stage0.PrepareConfig{
		CommonConfig: stage0.CommonConfig{
			Store:       s,
			Debug:       globalFlags.Debug,
			Stage1Image: *s1img,
			UUID:        p.uuid,
		},
		Ports:       []types.ExposedPort(flagPorts),
		InheritEnv:  flagInheritEnv,
		ExplicitEnv: flagExplicitEnv.Strings(),
		UseOverlay:  !flagNoOverlay && common.SupportsOverlay(),
		Apps:        &rktApps,
	}

	if err = stage0.Prepare(pcfg, p.path(), p.uuid); err != nil {
		stderr("prepare: error setting up stage0: %v", err)
		return 1
	}

	if err := p.sync(); err != nil {
		stderr("prepare: error syncing pod data: %v", err)
		return 1
	}

	if err := p.xToPrepared(); err != nil {
		stderr("prepare: error transitioning to prepared: %v", err)
		return 1
	}

	os.Stdout = origStdout // restore output in case of --quiet
	stdout("%s", p.uuid.String())

	return 0
}
