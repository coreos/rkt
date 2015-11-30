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
	"os"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/schema/types"
	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/spf13/cobra"
	"github.com/coreos/rkt/common"
	"github.com/coreos/rkt/pkg/lock"
	"github.com/coreos/rkt/pkg/uid"
	"github.com/coreos/rkt/stage0"
	"github.com/coreos/rkt/store"
)

var (
	cmdPrepare = &cobra.Command{
		Use:   "prepare [--volume=name,kind=host,...] [--mount volume=VOL,target=PATH] [--quiet] IMAGE [-- image-args...[---]]...",
		Short: "Prepare to run image(s) in a pod in rkt",
		Long: `IMAGE should be a string referencing an image; either a hash, local file on disk, or URL.
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

	addStage1ImageFlag(cmdPrepare.Flags())
	cmdPrepare.Flags().Var(&flagPorts, "port", "ports to expose on the host (needs contained networking with NAT)")
	cmdPrepare.Flags().BoolVar(&flagQuiet, "quiet", false, "suppress superfluous output on stdout, print only the UUID on success")
	cmdPrepare.Flags().BoolVar(&flagInheritEnv, "inherit-env", false, "inherit all environment variables not set by apps")
	cmdPrepare.Flags().BoolVar(&flagNoOverlay, "no-overlay", false, "disable overlay filesystem")
	cmdPrepare.Flags().BoolVar(&flagPrivateUsers, "private-users", false, "run within user namespaces (experimental).")
	cmdPrepare.Flags().Var(&flagExplicitEnv, "set-env", "an environment variable to set for apps in the form name=value")
	cmdPrepare.Flags().BoolVar(&flagStoreOnly, "store-only", false, "use only available images in the store (do not discover or download from remote URLs)")
	cmdPrepare.Flags().BoolVar(&flagNoStore, "no-store", false, "fetch images ignoring the local store")
	cmdPrepare.Flags().StringVar(&flagPodManifest, "pod-manifest", "", "the path to the pod manifest. If it's non-empty, then only '--quiet' and '--no-overlay' will have effects")
	cmdPrepare.Flags().Var((*appsVolume)(&rktApps), "volume", "volumes to make available in the pod")

	// per-app flags
	cmdPrepare.Flags().Var((*appExec)(&rktApps), "exec", "override the exec command for the preceding image")
	cmdPrepare.Flags().Var((*appMount)(&rktApps), "mount", "mount point binding a volume to a path within an app")

	// Disable interspersed flags to stop parsing after the first non flag
	// argument. This is need to permit to correctly handle
	// multiple "IMAGE -- imageargs ---"  options
	cmdPrepare.Flags().SetInterspersed(false)
}

func runPrepare(cmd *cobra.Command, args []string) (exit int) {
	var err error
	origStdout := os.Stdout
	privateUsers := uid.NewBlankUidRange()
	if flagQuiet {
		if os.Stdout, err = os.Open("/dev/null"); err != nil {
			stderr("prepare: unable to open /dev/null: %v", err)
			return 1
		}
	}

	if flagStoreOnly && flagNoStore {
		stderr("both --store-only and --no-store specified")
		return 1
	}

	if flagPrivateUsers {
		if !common.SupportsUserNS() {
			stderr("prepare: --private-users is not supported, kernel compiled without user namespace support")
			return 1
		}
		privateUsers.SetRandomUidRange(uid.DefaultRangeCount)
	}

	if err = parseApps(&rktApps, args, cmd.Flags(), true); err != nil {
		stderr("prepare: error parsing app image arguments: %v", err)
		return 1
	}

	if len(flagPodManifest) > 0 && (len(flagPorts) > 0 || flagInheritEnv || !flagExplicitEnv.IsEmpty() || flagStoreOnly || flagNoStore) {
		stderr("prepare: conflicting flags set with --pod-manifest (see --help)")
		return 1
	}

	if rktApps.Count() < 1 && len(flagPodManifest) == 0 {
		stderr("prepare: must provide at least one image or specify the pod manifest")
		return 1
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
			s:             s,
			headers:       config.AuthPerHost,
			dockerAuth:    config.DockerCredentialsPerRegistry,
			insecureFlags: globalFlags.InsecureFlags,
			debug:         globalFlags.Debug,
		},
		storeOnly: flagStoreOnly,
		noStore:   flagNoStore,
		withDeps:  false,
	}

	s1img, err := getStage1Hash(s, cmd)
	if err != nil {
		stderr("prepare: %v", err)
		return 1
	}

	fn.ks = getKeystore()
	fn.withDeps = true
	if err := fn.findImages(&rktApps); err != nil {
		stderr("prepare: %v", err)
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
		PrivateUsers: privateUsers,
	}

	if len(flagPodManifest) > 0 {
		pcfg.PodManifest = flagPodManifest
	} else {
		pcfg.Ports = []types.ExposedPort(flagPorts)
		pcfg.InheritEnv = flagInheritEnv
		pcfg.ExplicitEnv = flagExplicitEnv.Strings()
		pcfg.Apps = &rktApps
	}

	if globalFlags.Debug {
		stage0.InitDebug()
	}

	keyLock, err := lock.SharedKeyLock(lockDir(), common.PrepareLock)
	if err != nil {
		stderr("rkt: cannot get shared prepare lock: %v", err)
		return 1
	}
	if err = stage0.Prepare(pcfg, p.path(), p.uuid); err != nil {
		stderr("prepare: error setting up stage0: %v", err)
		keyLock.Close()
		return 1
	}
	keyLock.Close()

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
