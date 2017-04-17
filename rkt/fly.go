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
	"path/filepath"
	"syscall"

	"github.com/coreos/rkt/Godeps/_workspace/src/code.google.com/p/go-uuid/uuid"
	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/schema/types"
	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/spf13/cobra"
	"github.com/coreos/rkt/common/apps"
	"github.com/coreos/rkt/pkg/aci"
	"github.com/coreos/rkt/pkg/uid"
	"github.com/coreos/rkt/store"
)

var (
	cmdFly = &cobra.Command{
		Use:   "fly IMAGE [ -- image-args...]",
		Short: "Run a single application image with no pod or isolation",
		Long:  `IMAGE should be a string referencing an image; either a hash, local file on disk, or URL.`,
		Run:   runWrapper(runFly),
	}
)

func init() {
	cmdRkt.AddCommand(cmdFly)

	// Disable interspersed flags to stop parsing after the first non flag
	// argument. All the subsequent parsing will be done by parseApps.
	// This is needed to correctly handle image args
	cmdFly.Flags().SetInterspersed(false)
}

func runFlyPrepareApp(app *apps.Apps) (string, *types.App, error) {
	privateUsers := uid.NewBlankUidRange()

	s, err := store.NewStore(globalFlags.Dir)
	if err != nil {
		stderr("fly: cannot open store: %v", err)
		return "", nil, err
	}

	config, err := getConfig()
	if err != nil {
		stderr("fly: cannot get configuration: %v", err)
		return "", nil, err
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
		withDeps: true,
	}

	fn.ks = getKeystore()
	if err := fn.findImages(app); err != nil {
		stderr("fly: cannot find image: %v", err)
		return "", nil, err
	}

	u, err := types.NewUUID(uuid.New())
	if err != nil {
		stderr("fly: error creating UUID: %v", err)
		return "", nil, err
	}
	dir := filepath.Join(flightDir(), u.String())
	// TODO(jonboulle): lock this directory?
	// TODO(jonboulle): require parent dir to exist?
	err = os.MkdirAll(dir, 0700)
	if err != nil {
		stderr("fly: error creating directory: %v", err)
		return "", nil, err
	}

	rktApp := app.Last()
	id := rktApp.ImageID
	image, err := s.GetImageManifest(id.String())
	if err != nil {
		os.RemoveAll(dir)
		stderr("fly: error getting image manifest: %v", err)
		return "", nil, err
	}
	if image.App == nil {
		os.RemoveAll(dir)
		stderr("fly: image has no App section")
		return "", nil, err
	}

	//TODO(jonboulle): support overlay?
	err = aci.RenderACIWithImageID(id, dir, s, privateUsers)
	if err != nil {
		os.RemoveAll(dir)
		stderr("fly: error rendering ACI: %v", err)
		return "", nil, err
	}

	return dir, image.App, nil
}

func runFly(cmd *cobra.Command, args []string) (exit int) {
	var rktApp apps.Apps
	err := parseApps(&rktApp, args, cmd.Flags(), true)
	if err != nil {
		stderr("fly: error parsing app image arguments: %v", err)
		return 1
	}

	if rktApp.Count() != 1 {
		stderr("fly: must provide exactly one image")
		return 1
	}

	if globalFlags.Dir == "" {
		log.Printf("fly: dir unset - using temporary directory")
		var err error
		globalFlags.Dir, err = ioutil.TempDir("", "rkt")
		if err != nil {
			stderr("fly: error creating temporary directory: %v", err)
			return 1
		}
	}

	dir, imApp, err := runFlyPrepareApp(&rktApp)
	if err != nil {
		stderr("fly: error preparing App: %v", err)
		return 1
	}

	app := rktApp.Last()
	execargs := append(imApp.Exec, app.Args...)

	rfs := filepath.Join(dir, "rootfs")
	if err := os.Chdir(rfs); err != nil {
		os.RemoveAll(dir)
		stderr("fly: error changing directory: %v", err)
		return 1
	}
	if err := syscall.Chroot("."); err != nil {
		os.RemoveAll(dir)
		stderr("fly: error chrooting: %v", err)
		return 1
	}
	execPath := execargs[0]
	if _, err := os.Stat(execPath); err != nil {
		os.RemoveAll(dir)
		stderr("fly: error finding exec %v: %v", execPath, err)
		return 1
	}
	if err := syscall.Exec(execargs[0], execargs, os.Environ()); err != nil {
		os.RemoveAll(dir)
		stderr("fly: error execing: %v", err)
		return 1
	}
	// should never reach here
	panic("exec did not occur!")
}
