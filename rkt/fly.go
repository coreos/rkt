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
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/schema"
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
		Use:   "fly [--volume=name,kind=host,...] [--mount volume=VOL,target=PATH] IMAGE [-- image-args...]",
		Short: "Run a single application image with no pod or isolation",
		Long: `
IMAGE should be a string referencing an image; either a hash, local file on disk, or URL.

Volumes are made available to the container via --volume.
Mounts bind volumes into each image's root within the container via --mount.
`,
		Run: runWrapper(runFly),
	}
	flagExec string
)

type flyMount struct {
	HostPath         string
	TargetPrefixPath string
	RelTargetPath    string
	Fs               string
	Flags            uintptr
}

func init() {
	cmdRkt.AddCommand(cmdFly)

	cmdFly.Flags().Var((*appsVolume)(&rktApps), "volume", "volumes to make available in the pod")
	cmdFly.Flags().StringVar(&flagExec, "exec", "", "Override the executable")
	//cmdRun.Flags().Var((*appExec)(&rktApps), "exec", "override the exec command for the preceding image")
	cmdFly.Flags().Var((*appMount)(&rktApps), "mount", "mount point binding a volume to a path within an app")

	// Disable interspersed flags to stop parsing after the first non flag
	// argument. All the subsequent parsing will be done by parseApps.
	// This is needed to correctly handle image args
	cmdFly.Flags().SetInterspersed(false)

	// this ensures that main runs only on main thread (thread group leader).
	// since namespace ops (unshare, setns) are done for a single thread, we
	// must ensure that the goroutine does not jump from OS thread to thread
	runtime.LockOSThread()
}

func runFlyPrepareApp(apps *apps.Apps) (string, *types.App, error) {
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
			s:             s,
			headers:       config.AuthPerHost,
			dockerAuth:    config.DockerCredentialsPerRegistry,
			insecureFlags: globalFlags.InsecureFlags,
			debug:         globalFlags.Debug,
		},
		storeOnly: flagStoreOnly,
		noStore:   flagNoStore,
		withDeps:  false, // TODO? support dependencies
	}

	fn.ks = getKeystore()
	if err := fn.findImages(apps); err != nil {
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

	rktApp := apps.Last()
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
	err := parseApps(&rktApps, args, cmd.Flags(), true)
	if err != nil {
		stderr("fly: error parsing app image arguments: %v", err)
		return 1
	}

	if rktApps.Count() != 1 {
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

	dir, imApp, err := runFlyPrepareApp(&rktApps)
	if err != nil {
		stderr("fly: error preparing App: %v", err)
		return 1
	}

	app := rktApps.Last()
	var execargs []string
	if flagExec != "" {
		log.Printf("Overriding exec with %q", flagExec)
		execargs = []string{flagExec}
	} else {
		execargs = imApp.Exec
	}
	execargs = append(execargs, app.Args...)

	rfs := filepath.Join(dir, "rootfs")
	if _, err := os.Stat(rfs); os.IsNotExist(err) {
		stderr("fly: target root directory %q", rfs, err)
		os.RemoveAll(dir)
		return 1
	}

	type vmTuple struct {
		V types.Volume
		M schema.Mount
	}

	namedVolumeMounts := map[types.ACName]vmTuple{}

	// Insert the command-line Mounts
	for _, m := range rktApps.Mounts {
		_, exists := namedVolumeMounts[m.Volume]
		if exists {
			stderr("fly: duplicated mount given: %q", m.Volume)
			os.RemoveAll(dir)
			return 1
		}
		namedVolumeMounts[m.Volume] = vmTuple{M: m}
	}

	// Merge command-line Mounts with ImageManifest's MountPoints
	for _, mp := range imApp.MountPoints {
		tuple, exists := namedVolumeMounts[mp.Name]
		switch {
		case exists && tuple.M.Path != mp.Path:
			stderr("fly: conflicting path information from mount and mountpoint %q", mp.Name)
			os.RemoveAll(dir)
			return 1
		case !exists:
			namedVolumeMounts[mp.Name] = vmTuple{M: schema.Mount{Volume: mp.Name, Path: mp.Path}}
		}
	}

	// Insert the command-line Volumes
	for _, v := range rktApps.Volumes {
		// Check if we have a mount for this volume
		tuple, exists := namedVolumeMounts[v.Name]
		if !exists {
			stderr("fly: missing mount for volume %q", v.Name)
			os.RemoveAll(dir)
			return 1
		} else if tuple.M.Volume != v.Name {
			// TODO(steveeJ): remove this case. it's merely a safety mechanism regarding the implementation
			stderr("fly: mismatched volume:mount pair: %q != %q", v.Name, tuple.M.Volume)
			os.RemoveAll(dir)
			return 1
		}
		namedVolumeMounts[v.Name] = vmTuple{V: v, M: tuple.M}
	}

	// Merge command-line Volumes with ImageManifest's MountPoints
	for _, mp := range imApp.MountPoints {
		// Check if we have a volume for this mountpoint
		tuple, exists := namedVolumeMounts[mp.Name]
		if !exists || tuple.V.Name == "" {
			stderr("fly: missing volume for mountpoint %q", mp.Name)
			os.RemoveAll(dir)
			return 1
		}

		// If empty, fill in ReadOnly bit
		if tuple.V.ReadOnly == nil {
			v := tuple.V
			v.ReadOnly = &mp.ReadOnly
			namedVolumeMounts[mp.Name] = vmTuple{M: tuple.M, V: v}
		}
	}

	argFlyMounts := []flyMount{}
	for _, tuple := range namedVolumeMounts {
		var flags uintptr = syscall.MS_BIND | syscall.MS_REC
		if tuple.V.ReadOnly != nil && *tuple.V.ReadOnly {
			flags |= syscall.MS_RDONLY
		}
		argFlyMounts = append(
			argFlyMounts,
			flyMount{tuple.V.Source, rfs, tuple.M.Path, "none", flags},
		)
	}

	// create a separate mount namespace so the filesystems
	// are unmounted when exiting the pod
	if err := syscall.Unshare(syscall.CLONE_NEWNS); err != nil {
		stderr(fmt.Sprintf("fly: can not unshare: %v", err))
		os.RemoveAll(dir)
		return 1
	}

	// After this point we start to bind directories, so we can't simply RemoveAll(dir) anymore.
	// TODO: subscribe to the GC mechanism to have the directory cleaned up

	for _, mount := range append(
		[]flyMount{
			// we recursively make / a "shared and slave" so mount events from the
			// new namespace don't propagate to the host namespace but mount events
			// from the host propagate to the new namespace and are forwarded to
			// its peer group
			// See https://www.kernel.org/doc/Documentation/filesystems/sharedsubtree.txt
			{"", "", "/", "none", syscall.MS_REC | syscall.MS_SLAVE},
			{"", "", "/", "none", syscall.MS_REC | syscall.MS_SHARED},

			{rfs, rfs, "/", "none", syscall.MS_BIND | syscall.MS_REC},
			{"/dev", rfs, "/dev", "none", syscall.MS_BIND | syscall.MS_REC},
			{"/proc", rfs, "/proc", "none", syscall.MS_BIND | syscall.MS_REC},
			{"/sys", rfs, "/sys", "none", syscall.MS_BIND | syscall.MS_REC},
		},
		argFlyMounts...,
	) {
		var (
			err            error
			hostPathInfo   os.FileInfo
			targetPathInfo os.FileInfo
		)
		if mount.HostPath != "" {
			if hostPathInfo, err = os.Stat(mount.HostPath); err != nil {
				stderr("fly: something is wrong with the host directory %s: \n%v", mount.HostPath, err)
				return 1
			}
		}
		absTargetPath := filepath.Join(mount.TargetPrefixPath, mount.RelTargetPath)
		if absTargetPath != "/" {
			if targetPathInfo, err = os.Stat(absTargetPath); err != nil && !os.IsNotExist(err) {
				stderr("fly: something is wrong with the target directory %s: \n%v", absTargetPath, err)
				return 1
			}

			if targetPathInfo != nil {
				switch {
				case hostPathInfo.IsDir() && !targetPathInfo.IsDir():
					stderr("fly: can't mount:  %q is a directory while %q is not", mount.HostPath, absTargetPath)
					return 1
				case !hostPathInfo.IsDir() && targetPathInfo.IsDir():
					stderr("fly: can't mount:  %q is not a directory while %q is", mount.HostPath, absTargetPath)
					return 1
				}
			} else {
				absTargetPathParent, _ := filepath.Split(absTargetPath)
				if err := os.MkdirAll(absTargetPathParent, 0700); err != nil {
					stderr("fly: could not create directory %q: \n%v", absTargetPath, err)
					return 1
				}
				if hostPathInfo.IsDir() {
					if err := os.Mkdir(absTargetPath, 0700); err != nil {
						stderr("fly: could not create directory %q: \n%v", absTargetPath, err)
						return 1
					}
				} else {
					file, err := os.OpenFile(absTargetPath, os.O_CREATE, 0700)
					if err != nil {
						stderr("fly: could not create file %q: \n%v", absTargetPath, err)
						return 1
					}
					file.Close()
				}
			}
		}
		if err := syscall.Mount(mount.HostPath, absTargetPath, mount.Fs, mount.Flags, ""); err != nil {
			stderr("Error mounting %q on %q: %v", mount.HostPath, absTargetPath, err)
			return 1
		}
		if (mount.Flags & syscall.MS_RDONLY) != 0 {
			// Read-Only needs to be remounted
			if err := syscall.Mount("", absTargetPath, "", mount.Flags|syscall.MS_REMOUNT, ""); err != nil {
				stderr("Error remounting %q read-only: %v", absTargetPath, err)
				return 1
			}
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

	// TODO: change user according to the manifest

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
