// Copyright 2015 The rkt Authors
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
	"io/ioutil"
	"log"
	"os"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/schema/types"
	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/spf13/cobra"
	"github.com/coreos/rkt/common"
	"github.com/coreos/rkt/stage0"
	"github.com/coreos/rkt/store"
)

var (
	cmdPrepare = &cobra.Command{
		Use:   "prepare [--volume=name,kind=host,...] [--quiet] IMAGE [-- image-args...[---]]...",
		Short: "Prepare to run image(s) in a pod in rkt",
		Long: `Image should be a string referencing an image; either a hash, local file on disk, or URL.
They will be checked in that order and the first match will be used.

An "--" may be used to inhibit rkt prepare's parsing of subsequent arguments,
which will instead be appended to the preceding image app's exec arguments.
End the image arguments with a lone "---" to resume argument parsing.`,
		Run: runWrapper(runPrepare),
	}
	flagQuiet bool
)

func init() {
	cmdRkt.AddCommand(cmdPrepare)

	cmdPrepare.Flags().StringVar(&flagStage1Image, "stage1-image", defaultStage1Image, `image to use as stage1. Local paths and http/https URLs are supported. If empty, rkt will look for a file called "stage1.aci" in the same directory as rkt itself`)
	cmdPrepare.Flags().Var(&flagVolumes, "volume", "volumes to mount into the pod")
	cmdPrepare.Flags().Var(&flagPorts, "port", "ports to expose on the host (requires --private-net)")
	cmdPrepare.Flags().BoolVar(&flagQuiet, "quiet", false, "suppress superfluous output on stdout, print only the UUID on success")
	cmdPrepare.Flags().BoolVar(&flagInheritEnv, "inherit-env", false, "inherit all environment variables not set by apps")
	cmdPrepare.Flags().BoolVar(&flagNoOverlay, "no-overlay", false, "disable overlay filesystem")
	cmdPrepare.Flags().Var(&flagExplicitEnv, "set-env", "an environment variable to set for apps in the form name=value")
	cmdPrepare.Flags().BoolVar(&flagLocal, "local", false, "use only local images (do not discover or download from remote URLs)")
	cmdPrepare.Flags().StringVar(&flagPodManifest, "pod-manifest", "", "the path to the pod manifest. If it's non-empty, then only '--quiet' and '--no-overlay' will have effects")
	cmdPrepare.Flags().StringVar(&flagPrivateUsers, "private-users", "", "Run within user namespace. Can be set to [=UIDBASE[:NUIDS]]")

	// Disable interspersed flags to stop parsing after the first non flag
	// argument. This is need to permit to correctly handle
	// multiple "IMAGE -- imageargs ---"  options
	cmdPrepare.Flags().SetInterspersed(false)
}

func runPrepare(cmd *cobra.Command, args []string) (exit int) {
	var err error
	origStdout := os.Stdout
	if flagQuiet {
		if os.Stdout, err = os.Open("/dev/null"); err != nil {
			stderr("prepare: unable to open /dev/null")
			return 1
		}
	}

	if err = parseApps(&rktApps, args, cmd.Flags(), true); err != nil {
		stderr("prepare: error parsing app image arguments: %v", err)
		return 1
	}

	if len(flagPodManifest) > 0 && (len(flagVolumes) > 0 || len(flagPorts) > 0 || flagInheritEnv || !flagExplicitEnv.IsEmpty() || flagLocal) {
		stderr("prepare: conflicting flags set with --pod-manifest (see --help)")
		return 1
	}

	if rktApps.Count() < 1 && len(flagPodManifest) == 0 {
		stderr("prepare: must provide at least one image or specify the pod manifest")
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
		local:    flagLocal,
		withDeps: false,
	}
	s1img, err := fn.findImage(flagStage1Image, "", false)
	if err != nil {
		stderr("prepare: finding stage1 image %q: %v", flagStage1Image, err)
		return 1
	}

	fn.ks = getKeystore()
	fn.withDeps = true
	if err := fn.findImages(&rktApps); err != nil {
		stderr("%v", err)
		return 1
	}

	p, err := newPod()
	if err != nil {
		stderr("prepare: error creating new pod: %v", err)
		return 1
	}

	cfg := stage0.CommonConfig{
		Store:       s,
		Stage1Image: *s1img,
		UUID:        p.uuid,
		Debug:       globalFlags.Debug,
	}

	pcfg := stage0.PrepareConfig{
		CommonConfig: cfg,
		UseOverlay:   !flagNoOverlay && common.SupportsOverlay(),
		PrivateUsers: flagPrivateUsers,
	}

	if len(flagPodManifest) > 0 {
		pcfg.PodManifest = flagPodManifest
	} else {
		pcfg.Volumes = []types.Volume(flagVolumes)
		pcfg.Ports = []types.ExposedPort(flagPorts)
		pcfg.InheritEnv = flagInheritEnv
		pcfg.ExplicitEnv = flagExplicitEnv.Strings()
		pcfg.Apps = &rktApps
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
