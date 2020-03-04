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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/rkt/rkt/common"
	"github.com/rkt/rkt/pkg/fileutil"
	"github.com/rkt/rkt/pkg/fs"
	"github.com/rkt/rkt/pkg/user"
	stage1types "github.com/rkt/rkt/stage1/common/types"

	"github.com/appc/spec/schema"
	"github.com/appc/spec/schema/types"
	"github.com/hashicorp/errwrap"
)

/*
 * Some common stage1 mount tasks
 *
 * TODO(cdc) De-duplicate code from stage0/gc.go
 */

// Mount extends schema.Mount with additional rkt specific fields.
type Mount struct {
	schema.Mount

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

// Source computes the real volume source for a volume.
// Volumes of type 'empty' use a workdir relative to podRoot
func (m *Mount) Source(podRoot string) string {
	switch m.Volume.Kind {
	case "host":
		return m.Volume.Source
	case "empty":
		return filepath.Join(common.SharedVolumesPath(podRoot), m.Volume.Name.String())
	}
	return "" // We validate in RuntimeMounts that it's valid
}

type (
	// volumesMap is a mapping of volume name to volume details.
	volumesMap map[types.ACName]types.Volume

	// mountsMap is a mapping of mount point name to mount point details.
	mountsMap map[types.ACName]types.MountPoint
)

// rtmState is a state of the POD manifest traverse though the volumes
// configuration of the runtime application. It holds the mappings of
// of volumes and mounts for fast access.
type rtmState struct {
	// Configuration of the runtime mounts.
	rtm *RuntimeMounts

	// processed defines a set of processed mount points, so we don't
	// have to walk though them multiple times.
	processed map[string]struct{}

	// The mappings of the POD volumes and the application mount points
	// for the fast access.
	volumes volumesMap
	mounts  mountsMap
}

// newRtmState returns a new instance of the runtime-mounts state.
func newRtmState(rtm *RuntimeMounts) *rtmState {
	return &rtmState{rtm: rtm, processed: make(map[string]struct{})}
}

// Mark the specified path as processed.
func (s *rtmState) process(path string) {
	s.processed[path] = struct{}{}
}

// isProcessed returns true when the path have been already processed
// and false otherwise.
func (s *rtmState) isProcessed(path string) bool {
	_, ok := s.processed[path]
	return ok
}

// mountPointOf returns a mount point of the corresponding volume with
// an indicator of mount point existence.
//
// This method used for a lazy evaluation of the mapping of mount-point
// names to the mount-point details.
func (s *rtmState) mountPointOf(vol types.ACName) (types.MountPoint, bool) {
	if s.mounts == nil {
		app := s.rtm.Application.App
		s.mounts = make(mountsMap, len(app.MountPoints))

		for _, mp := range app.MountPoints {
			s.mounts[mp.Name] = mp
		}
	}

	mp, ok := s.mounts[vol]
	return mp, ok
}

// volumeOf returns a volume details of the POD with an indicator of
// volume existence.
func (s *rtmState) volumeOf(vol types.ACName) (types.Volume, bool) {
	if s.volumes == nil {
		s.volumes = make(volumesMap, len(s.rtm.Volumes))
		for _, v := range s.rtm.Volumes {
			s.volumes[v.Name] = v
		}
	}

	v, ok := s.volumes[vol]
	return v, ok
}

// RuntimeMounts represents a mount points generator of the runtime
// application.
type RuntimeMounts struct {
	// Application is the runtime application to generate mount points.
	Application *schema.RuntimeApp

	// Volumes specifies a list of volumes of the application.
	Volumes []types.Volume

	// DockerImplicit indicates if the resulting mount points should be
	// marked as implicit empty volumes from a Docker image.
	DockerImplicit bool
}

// NewRuntimeMounts creates a new instance of the RuntimeMounts with a
// attributes retrieved from the given POD configuration.
func NewRuntimeMounts(ra *schema.RuntimeApp, pod *stage1types.Pod) *RuntimeMounts {
	return &RuntimeMounts{
		Application: ra,
		Volumes:     pod.Manifest.Volumes,
		DockerImplicit: ConvertedFromDocker(
			pod.Images[ra.Name.String()],
		),
	}
}

// emptyVolume returns a new instance of the empty volume with a unique
// name and default configuration.
func (rtm *RuntimeMounts) emptyVolume(mpoint *types.MountPoint) types.Volume {
	defaultMode := "0755"
	uniqName := rtm.Application.Name + "-" + mpoint.Name

	return types.Volume{
		Name: uniqName,
		Kind: "empty",
		Mode: &defaultMode,
		UID:  new(int),
		GID:  new(int),
	}
}

// traverseMounts builds a mapping of the mounts of the runtime application
// to the volumes declared in a POD manifest. For each successfully created
// mapping function executes a specified call-back function for further
// processing.
//
// An error returns, when runtime application declares a mount for non-existing
// volume (i.e. POD manifest is missing the respective volume declaration).
func (rtm *RuntimeMounts) traverseMounts(s *rtmState, f func(m Mount) error) error {
	// Check runtimeApp's Mounts
	for _, m := range rtm.Application.Mounts {
		vol := m.AppVolume // Mounts can supply a volume
		if vol == nil {
			vv, ok := s.volumeOf(m.Volume)
			if !ok {
				return fmt.Errorf("could not find volume %s", m.Volume)
			}
			vol = &vv
		}

		// Find a corresponding MountPoint, which is optional
		mpoint, _ := s.mountPointOf(m.Volume)
		ro := mpoint.ReadOnly

		if vol.ReadOnly != nil {
			ro = *vol.ReadOnly
		}

		switch vol.Kind {
		case "host", "empty":
		default:
			const text = "Volume %s has invalid kind %s"
			return fmt.Errorf(text, vol.Name, vol.Kind)
		}

		mount := Mount{m, *vol, false, ro}
		if err := f(mount); err != nil {
			return err
		}

		s.process(m.Path)
	}

	return nil
}

// traverseMountPoints builds a mapping of the mount points of runtime
// application that does not declare a corresponding mount in a manifest.
//
// In order to preserve compatibility with Docker images, applications are
// allowed to declare mounts without corresponsind mount points, therefore,
// it is required to generate an empty volume for each unmapped mount.
//
// For each mount successfully created mapping function executes a specified
// call-back function for further processing.
func (rtm *RuntimeMounts) traverseMountPoints(s *rtmState, f func(Mount) error) error {
	var mount Mount

	// Now, match up MountPoints with Mounts or Volumes
	// If there's no Mount and no Volume, generate an empty volume
	for _, mp := range rtm.Application.App.MountPoints {
		// there's already a Mount for this MountPoint, stop
		if s.isProcessed(mp.Path) {
			continue
		}

		// No Mount, try to match based on volume name
		vol, ok := s.volumeOf(mp.Name)
		// there is no volume for this mount point, creating an "empty" volume
		// implicitly
		if !ok {
			log.Printf("warning: no volume specified for mount point %q, implicitly creating an \"empty\" volume. This volume will be removed when the pod is garbage-collected.", mp.Name)

			if rtm.DockerImplicit {
				log.Printf("Docker converted image, initializing implicit volume with data contained at the mount point %q.", mp.Name)
			}

			emptyVol := rtm.emptyVolume(&mp)
			s.volumes[emptyVol.Name] = emptyVol

			acm := schema.Mount{Volume: emptyVol.Name, Path: mp.Path}
			mount = Mount{acm, emptyVol, rtm.DockerImplicit, mp.ReadOnly}
		} else {
			ro := mp.ReadOnly
			if vol.ReadOnly != nil {
				ro = *vol.ReadOnly
			}

			acm := schema.Mount{Volume: vol.Name, Path: mp.Path}
			mount = Mount{acm, vol, false, ro}
		}

		if err := f(mount); err != nil {
			return err
		}
	}

	return nil
}

// Mounts maps mount-point paths to volumes, returning a list of mounts.
func (rtm *RuntimeMounts) Mounts() ([]Mount, error) {
	var mountPoints []Mount

	err := rtm.MountsFunc(func(m Mount) error {
		mountPoints = append(mountPoints, m)
		return nil
	})

	// Return a nil slice in case of the error happened during the
	// iteration over the mount points of the runtime application.
	if err != nil {
		mountPoints = nil
	}

	return mountPoints, err
}

// MountsFunc maps mount-point path to volumes and applies a given
// function to it.
func (rtm *RuntimeMounts) MountsFunc(f func(Mount) error) error {
	// RuntimeApps have mounts, whereas Apps have mountPoints; mountPoints
	// are partially for Docker compat; since apps can declare mountpoints.
	//
	// However, if we just run with rkt run, then we'll only have a Mount
	// and no corresponding MountPoint. Furthermore, Mounts can have embedded
	// volumes in the case of the CRI.
	//
	// So, we generate a pile of Mounts and their corresponding Volume

	st := newRtmState(rtm)
	if err := rtm.traverseMounts(st, f); err != nil {
		return err
	}

	return rtm.traverseMountPoints(st, f)
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
func BindMount(mnt fs.MountUnmounter, source, destination string, readOnly bool) error {
	absSource, err := filepath.EvalSymlinks(source)
	if err != nil {
		return errwrap.Wrap(fmt.Errorf("Could not resolve symlink for source %v", source), err)
	}

	if err := EnsureTargetExists(absSource, destination); err != nil {
		return errwrap.Wrap(fmt.Errorf("Could not create destination mount point: %v", destination), err)
	} else if err := mnt.Mount(absSource, destination, "bind", syscall.MS_BIND, ""); err != nil {
		return errwrap.Wrap(fmt.Errorf("Could not bind mount %v to %v", absSource, destination), err)
	}
	if readOnly {
		err := mnt.Mount(source, destination, "bind", syscall.MS_REMOUNT|syscall.MS_RDONLY|syscall.MS_BIND, "")

		// If we failed to remount ro, unmount
		if err != nil {
			mnt.Unmount(destination, 0) // if this fails, oh well
			return errwrap.Wrap(fmt.Errorf("Could not remount %v read-only", destination), err)
		}
	}
	return nil
}

// EnsureTargetExists will recursively create a given mountpoint. If directories
// are created, their permissions are initialized to common.SharedVolumePerm
func EnsureTargetExists(source, destination string) error {
	fileInfo, err := os.Stat(source)
	if err != nil {
		return errwrap.Wrap(fmt.Errorf("could not stat source location: %v", source), err)
	}

	targetPathParent, _ := filepath.Split(destination)
	if err := os.MkdirAll(targetPathParent, common.SharedVolumePerm); err != nil {
		return errwrap.Wrap(fmt.Errorf("could not create parent directory: %v", targetPathParent), err)
	}

	if fileInfo.IsDir() {
		if err := os.Mkdir(destination, common.SharedVolumePerm); err != nil && !os.IsExist(err) {
			return errwrap.Wrap(errors.New("could not create destination directory "+destination), err)
		}
	} else {
		if file, err := os.OpenFile(destination, os.O_CREATE, common.SharedVolumePerm); err != nil {
			return errwrap.Wrap(errors.New("could not create destination file"), err)
		} else {
			file.Close()
		}
	}
	return nil
}
