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

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/schema/types"
	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/pborman/uuid"
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
		storeOnly: flagStoreOnly,
		noStore:   flagNoStore,
		withDeps:  false,
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
	if _, err := os.Stat(rfs); os.IsNotExist(err) {
		stderr("fly: target root directory %q", rfs, err)
		return 1
	}

	// create a separate mount namespace so the filesystems
	// are unmounted when exiting the pod
	if err := syscall.Unshare(syscall.CLONE_NEWNS); err != nil {
		log.Fatalf("Error unsharing: %v", err)
	}

	for _, mount := range []struct {
		HostPath         string
		TargetPrefixPath string
		RelTargetPath    string
		Fs               string
		Flags            uintptr
	}{
		// we recursively make / a "shared and slave" so mount events from the
		// new namespace don't propagate to the host namespace but mount events
		// from the host propagate to the new namespace and are forwarded to
		// its peer group
		// See https://www.kernel.org/doc/Documentation/filesystems/sharedsubtree.txt
		{"", "", "/", "none", syscall.MS_REC | syscall.MS_SLAVE},
		{"", "", "/", "none", syscall.MS_REC | syscall.MS_SHARED},

		{"/dev", rfs, "/dev", "none", syscall.MS_BIND | syscall.MS_REC},
		{"/proc", rfs, "/proc", "none", syscall.MS_BIND | syscall.MS_REC},
		{"/sys", rfs, "/sys", "none", syscall.MS_BIND | syscall.MS_REC},
		{"/", rfs, "/host", "none", syscall.MS_BIND | syscall.MS_REC | syscall.MS_RDONLY},
	} {
		absTargetPath := filepath.Join(mount.TargetPrefixPath, mount.RelTargetPath)
		if _, err := os.Stat(absTargetPath); os.IsNotExist(err) {
			if err := os.Mkdir(absTargetPath, 0700); err != nil {
				stderr("fly: could not create directory %q: \n%v", absTargetPath, err)
				return 1
			}
		}
		if err := syscall.Mount(mount.HostPath, absTargetPath, mount.Fs, mount.Flags, ""); err != nil {
			log.Fatalf("Error mounting %q on %q: %v", mount.HostPath, absTargetPath, err)
		}
	}

	if err := syscall.Chroot(rfs); err != nil {
		stderr("fly: error chrooting: %v", err)
		return 1
	}

	if err := os.Chdir("/"); err != nil {
		stderr("fly: couldn't change to root new directory: %v", err)
		return 1
	}

	// TODO: change user

	execPath := execargs[0]
	if _, err := os.Stat(execPath); err != nil {
		stderr("fly: error finding exec %v: %v", execPath, err)
		return 1
	}

	// TODO: insert environment from manifest
	environ := []string{"PATH=/bin:/sbin:/usr/bin:/usr/local/bin"}

	// should never reach here
	if err := syscall.Exec(execPath, execargs, environ); err != nil {
		stderr("fly: error executing app: %v", err)
		return 1
	}
	panic("exec did not occur!")

	// TODO: explore this cleanup route
	// * wait for app
	// * ForkExec ourselves with cleanup flag and directory (is this enough to have the kernel unmount the mounts?)
	// * forked' version only does cleanup
	//appCmd := exec.Command(execargs[0], execargs...)
	//appCmd.Env = environ
	//appCmd.Stdout = os.Stdout
	//appCmd.Stdin = os.Stdin
	//appCmd.Stderr = os.Stderr
	//if err := appCmd.Run(); err != nil {
	//	stderr("fly: error running app: %v", err)
	//	return 1
	//}
	//return 0
}
