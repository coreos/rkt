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

package common

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/coreos/rkt/pkg/fileutil"
	"github.com/coreos/rkt/pkg/user"

	"github.com/appc/spec/schema"
	"github.com/appc/spec/schema/types"
	"github.com/hashicorp/errwrap"

	stage1commontypes "github.com/coreos/rkt/stage1/common/types"
)

/*
 * Some common stage1 mount tasks
 *
 * TODO(cdc) De-duplicate code from stage0/gc.go
 */

// mountWrapper is a wrapper around a schema.Mount with an additional field indicating
// whether it is an implicit empty volume converted from a Docker image.
type mountWrapper struct {
	Mount          schema.Mount
	Volume         types.Volume
	DockerImplicit bool
	ReadOnly       bool
}

// ConvertedFromDocker determines if an app's image has been converted
// from docker. This is needed because implicit docker empty volumes have
// different behavior from AppC
func ConvertedFromDocker(im *schema.ImageManifest) bool {
	if im == nil { // nil sometimes sneaks in here due to unit tests
		return false
	}
	ann := im.Annotations
	_, ok := ann.Get("appc.io/docker/repository")
	return ok
}

// GenerateMounts maps MountPoint paths to volumes, returning a list of mounts,
// each with a parameter indicating if it's an implicit empty volume from a
// Docker image.
func GenerateMounts(ra *schema.RuntimeApp, podVolumes []types.Volume, convertedFromDocker bool) ([]mountWrapper, error) {
	app := ra.App

	var genMnts []mountWrapper

	vols := make(map[types.ACName]types.Volume)
	for _, v := range podVolumes {
		vols[v.Name] = v
	}

	// RuntimeApps have mounts, whereas Apps have mountPoints. mountPoints are partially for
	// Docker compat; since apps can declare mountpoints. However, if we just run with rkt run,
	// then we'll only have a Mount and no corresponding MountPoint.
	// Furthermore, Mounts can have embedded volumes in the case of the CRI.
	// So, we generate a pile of Mounts and their corresponding Volume

	// Map of hostpath -> Mount
	mnts := make(map[string]schema.Mount)

	// Check runtimeApp's Mounts
	for _, m := range ra.Mounts {
		mnts[m.Path] = m

		vol := m.AppVolume // Mounts can supply a volume
		if vol == nil {
			vv, ok := vols[m.Volume]
			if !ok {
				return nil, fmt.Errorf("could not find volume %s", m.Volume)
			}
			vol = &vv
		}

		// Find a corresponding MountPoint, which is optional
		ro := false
		for _, mp := range ra.App.MountPoints {
			if mp.Name == m.Volume {
				ro = mp.ReadOnly
				break
			}
		}
		if vol.ReadOnly != nil {
			ro = *vol.ReadOnly
		}

		genMnts = append(genMnts,
			mountWrapper{
				Mount:          m,
				DockerImplicit: false,
				ReadOnly:       ro,
				Volume:         *vol,
			})
	}

	// Now, match up MountPoints with Mounts or Volumes
	// If there's no Mount and no Volume, generate an empty volume
	for _, mp := range app.MountPoints {
		// there's already a Mount for this MountPoint, stop
		if _, ok := mnts[mp.Path]; ok {
			continue
		}

		// No Mount, try to match based on volume name
		vol, ok := vols[mp.Name]
		// there is no volume for this mount point, creating an "empty" volume
		// implicitly
		if !ok {
			defaultMode := "0755"
			defaultUID := 0
			defaultGID := 0
			uniqName := ra.Name + "-" + mp.Name
			emptyVol := types.Volume{
				Name: uniqName,
				Kind: "empty",
				Mode: &defaultMode,
				UID:  &defaultUID,
				GID:  &defaultGID,
			}

			log.Printf("warning: no volume specified for mount point %q, implicitly creating an \"empty\" volume. This volume will be removed when the pod is garbage-collected.", mp.Name)
			if convertedFromDocker {
				log.Printf("Docker converted image, initializing implicit volume with data contained at the mount point %q.", mp.Name)
			}

			vols[uniqName] = emptyVol
			genMnts = append(genMnts,
				mountWrapper{
					Mount: schema.Mount{
						Volume: uniqName,
						Path:   mp.Path,
					},
					Volume:         emptyVol,
					ReadOnly:       mp.ReadOnly,
					DockerImplicit: convertedFromDocker,
				})
		} else {
			ro := mp.ReadOnly
			if vol.ReadOnly != nil {
				ro = *vol.ReadOnly
			}
			genMnts = append(genMnts,
				mountWrapper{
					Mount: schema.Mount{
						Volume: vol.Name,
						Path:   mp.Path,
					},
					Volume:         vol,
					ReadOnly:       ro,
					DockerImplicit: false,
				})
		}
	}

	return genMnts, nil
}

// PrepareMountpoints creates and sets permissions for empty volumes.
// If the mountpoint comes from a Docker image and it is an implicit empty
// volume, we copy files from the image to the volume, see
// https://docs.docker.com/engine/userguide/containers/dockervolumes/#data-volumes
func PrepareMountpoints(volPath string, targetPath string, vol *types.Volume, dockerImplicit bool) error {
	if vol.Kind != "empty" {
		return nil
	}

	diag.Printf("creating an empty volume folder for sharing: %q", volPath)
	m, err := strconv.ParseUint(*vol.Mode, 8, 32)
	if err != nil {
		return errwrap.Wrap(fmt.Errorf("invalid mode %q for volume %q", *vol.Mode, vol.Name), err)
	}
	mode := os.FileMode(m)
	Uid := *vol.UID
	Gid := *vol.GID

	if dockerImplicit {
		fi, err := os.Stat(targetPath)
		if err == nil {
			// the directory exists in the image, let's set the same
			// permissions and copy files from there to the empty volume
			mode = fi.Mode()
			Uid = int(fi.Sys().(*syscall.Stat_t).Uid)
			Gid = int(fi.Sys().(*syscall.Stat_t).Gid)

			if err := fileutil.CopyTree(targetPath, volPath, user.NewBlankUidRange()); err != nil {
				return errwrap.Wrap(fmt.Errorf("error copying image files to empty volume %q", volPath), err)
			}
		}
	}

	if err := os.MkdirAll(volPath, 0770); err != nil {
		return errwrap.Wrap(fmt.Errorf("error creating %q", volPath), err)
	}
	if err := os.Chown(volPath, Uid, Gid); err != nil {
		return errwrap.Wrap(fmt.Errorf("could not change owner of %q", volPath), err)
	}
	if err := os.Chmod(volPath, mode); err != nil {
		return errwrap.Wrap(fmt.Errorf("could not change permissions of %q", volPath), err)
	}

	return nil
}

// BindMount, well, bind mounts a source in to a destination. This will
// do some bookkeeping:
// * evaluate all symlinks
// * ensure the source exists
// * recursively create the destination
func BindMount(source, destination string, readOnly bool) error {
	absSource, err := filepath.EvalSymlinks(source)
	if err != nil {
		return errwrap.Wrap(fmt.Errorf("Could not resolve symlink for source %v", source), err)
	}

	if err := ensureDestinationExists(absSource, destination); err != nil {
		return errwrap.Wrap(fmt.Errorf("Could not create destination mount point: %v", destination), err)
	} else if err := syscall.Mount(absSource, destination, "bind", syscall.MS_BIND, ""); err != nil {
		return errwrap.Wrap(fmt.Errorf("Could not bind mount %v to %v", absSource, destination), err)
	}
	if readOnly {
		err := syscall.Mount(source, destination, "bind", syscall.MS_REMOUNT|syscall.MS_RDONLY|syscall.MS_BIND, "")

		// If we failed to remount ro, unmount
		if err != nil {
			syscall.Unmount(destination, 0) // if this fails, oh well
			return errwrap.Wrap(fmt.Errorf("Could not remount %v read-only", destination), err)
		}
	}
	return nil
}

// ensureDestinationExists will recursively create a given mountpoint. If directories
// are created, their permissions are initialized to stage1/init/common.SharedVolPerm
func ensureDestinationExists(source, destination string) error {
	fileInfo, err := os.Stat(source)
	if err != nil {
		return errwrap.Wrap(fmt.Errorf("could not stat source location: %v", source), err)
	}

	targetPathParent, _ := filepath.Split(destination)
	if err := os.MkdirAll(targetPathParent, SharedVolPerm); err != nil {
		return errwrap.Wrap(fmt.Errorf("could not create parent directory: %v", targetPathParent), err)
	}

	if fileInfo.IsDir() {
		if err := os.Mkdir(destination, SharedVolPerm); !os.IsExist(err) {
			return err
		}
	} else {
		if file, err := os.OpenFile(destination, os.O_CREATE, SharedVolPerm); err != nil {
			return err
		} else {
			file.Close()
		}
	}
	return nil
}

func AppAddMounts(p *stage1commontypes.Pod, ra *schema.RuntimeApp, enterCmd []string) {
	vols := make(map[types.ACName]types.Volume)
	for _, v := range p.Manifest.Volumes {
		vols[v.Name] = v
	}

	imageManifest := p.Images[ra.Name.String()]

	mounts, err := GenerateMounts(ra, p.Manifest.Volumes, ConvertedFromDocker(imageManifest))
	if err != nil {
		log.FatalE("Could not generate mounts", err)
		os.Exit(1)
	}

	for _, m := range mounts {
		AppAddOneMount(p, ra, m.Volume.Source, m.Mount.Path, m.ReadOnly, enterCmd)
	}
}

/* AppAddOneMount bind-mounts "sourcePath" from the host into "dstPath" in
 * the container.
 *
 * We use the propagation mechanism of systemd-nspawn. In all systemd-nspawn
 * containers, the directory "/run/systemd/nspawn/propagate/$MACHINE_ID" on
 * the host is propagating mounts to the directory
 * "/run/systemd/nspawn/incoming/" in the container mount namespace. Once a
 * bind mount is propagated, we simply move to its correct location.
 *
 * The algorithm is the same as in "machinectl bind":
 * https://github.com/systemd/systemd/blob/v231/src/machine/machine-dbus.c#L865
 * except that we don't use setns() to enter the mount namespace of the pod
 * because Linux does not allow multithreaded applications (such as Go
 * programs) to change mount namespaces with setns. Instead, we fork another
 * process written in C (single-threaded) to enter the mount namespace. The
 * command used is specified by the "enterCmd" parameter.
 *
 * Users might request a bind mount to be set up read-only. This complicates
 * things a bit because on Linux, setting up a read-only bind mount involves
 * two mount() calls, so it is not atomic. We don't want the container to see
 * the mount in read-write mode, even for a short time, so we don't create the
 * bind mount directly in "/run/systemd/nspawn/propagate/$MACHINE_ID" to avoid
 * an immediate propagation to the container. Instead, we create a temporary
 * playground in "/tmp/rkt.propagate.XXXX" and create the bind mount in
 * "/tmp/rkt.propagate.XXXX/mount" with the correct read-only attribute before
 * moving it.
 *
 * Another complication is that the playground cannot be on a shared mount
 * because Linux does not allow MS_MOVE to be applied to mounts with MS_SHARED
 * parent mounts. But by default, systemd mounts everything as shared, see:
 * https://github.com/systemd/systemd/blob/v231/src/core/mount-setup.c#L392
 * We set up the temporary playground as a slave bind mount to avoid this
 * limitation.
 */
func AppAddOneMount(p *stage1commontypes.Pod, ra *schema.RuntimeApp, sourcePath string, dstPath string, readOnly bool, enterCmd []string) {
	/* The general plan:
	 * - bind-mount sourcePath to mountTmp
	 * - MS_MOVE mountTmp to mountOutside, the systemd propagate dir
	 * - systemd moves mountOutside to mountInside
	 * - in the stage1 namespace, bind mountInside to the app's rootfs
	 */

	/* Prepare a temporary playground that is not a shared mount */
	playgroundMount, err := ioutil.TempDir("", "rkt.propagate.")
	if err != nil {
		log.FatalE("error creating temporary propagation directory", err)
		os.Exit(1)
	}
	defer os.Remove(playgroundMount)

	err = syscall.Mount(playgroundMount, playgroundMount, "bind", syscall.MS_BIND, "")
	if err != nil {
		log.FatalE("error mounting temporary directory", err)
		os.Exit(1)
	}
	defer syscall.Unmount(playgroundMount, 0)

	err = syscall.Mount("", playgroundMount, "none", syscall.MS_SLAVE, "")
	if err != nil {
		log.FatalE("error mounting temporary directory", err)
		os.Exit(1)
	}

	/* Bind mount the source into the playground, possibly read-only */
	mountTmp := filepath.Join(playgroundMount, "mount")
	if err := ensureDestinationExists(sourcePath, mountTmp); err != nil {
		log.FatalE("error creating temporary mountpoint", err)
		os.Exit(1)
	}
	defer os.Remove(mountTmp)

	err = syscall.Mount(sourcePath, mountTmp, "bind", syscall.MS_BIND, "")
	if err != nil {
		log.FatalE("error mounting temporary mountpoint", err)
		os.Exit(1)
	}
	defer syscall.Unmount(mountTmp, 0)

	if readOnly {
		err = syscall.Mount("", mountTmp, "bind", syscall.MS_REMOUNT|syscall.MS_RDONLY|syscall.MS_BIND, "")
		if err != nil {
			log.FatalE("error remounting temporary mountpoint read-only", err)
			os.Exit(1)
		}
	}

	/* Now that the bind mount has the correct attributes (RO or RW), move
	 * it to the propagation directory prepared by systemd-nspawn */
	mountOutside := filepath.Join("/run/systemd/nspawn/propagate/", "rkt-"+p.UUID.String(), "rkt.mount")
	mountInside := filepath.Join("/run/systemd/nspawn/incoming/", filepath.Base(mountOutside))
	if err := ensureDestinationExists(sourcePath, mountOutside); err != nil {
		log.FatalE("error creating propagate mountpoint", err)
		os.Exit(1)
	}
	defer os.Remove(mountOutside)

	err = syscall.Mount(mountTmp, mountOutside, "", syscall.MS_MOVE, "")
	if err != nil {
		log.FatalE("error moving temporary mountpoint to propagate directory", err)
		os.Exit(1)
	}
	defer syscall.Unmount(mountOutside, 0)

	/* Finally move the bind mount at the correct place inside the container. */
	mountDst := filepath.Join("/opt/stage2", ra.Name.String(), "rootfs", dstPath)
	mountDstOutside := filepath.Join(p.Root, "stage1/rootfs", mountDst)
	if err := ensureDestinationExists(sourcePath, mountDstOutside); err != nil {
		log.FatalE("error creating destination directory", err)
		os.Exit(1)
	}

	args := enterCmd
	args = append(args, "/bin/mount", "--move", mountInside, mountDst)

	cmd := exec.Cmd{
		Path: args[0],
		Args: args,
	}

	if err := cmd.Run(); err != nil {
		log.PrintE("error executing mount move", err)
		os.Exit(1)
	}
}
