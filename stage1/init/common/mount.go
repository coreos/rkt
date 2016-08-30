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
	"os"
	"strconv"
	"syscall"

	"github.com/coreos/rkt/pkg/fileutil"
	"github.com/coreos/rkt/pkg/user"

	"github.com/appc/spec/schema"
	"github.com/appc/spec/schema/types"
	"github.com/hashicorp/errwrap"
)

// mountWrapper is a wrapper around a schema.Mount with an additional field indicating
// whether it is an implicit empty volume converted from a Docker image.
type mountWrapper struct {
	schema.Mount
	DockerImplicit bool
}

func isMPReadOnly(mountPoints []types.MountPoint, name types.ACName) bool {
	for _, mp := range mountPoints {
		if mp.Name == name {
			return mp.ReadOnly
		}
	}

	return false
}

// IsMountReadOnly returns if a mount should be readOnly.
// If the readOnly flag in the pod manifest is not nil, it overrides the
// readOnly flag in the image manifest.
func IsMountReadOnly(vol types.Volume, mountPoints []types.MountPoint) bool {
	if vol.ReadOnly != nil {
		return *vol.ReadOnly
	}

	return isMPReadOnly(mountPoints, vol.Name)
}

func convertedFromDocker(im *schema.ImageManifest) bool {
	ann := im.Annotations
	_, ok := ann.Get("appc.io/docker/repository")
	return ok
}

// GenerateMounts maps MountPoint paths to volumes, returning a list of mounts,
// each with a parameter indicating if it's an implicit empty volume from a
// Docker image.
func GenerateMounts(ra *schema.RuntimeApp, volumes map[types.ACName]types.Volume, imageManifest *schema.ImageManifest) []mountWrapper {
	app := ra.App

	var genMnts []mountWrapper

	mnts := make(map[string]schema.Mount)
	for _, m := range ra.Mounts {
		mnts[m.Path] = m
		genMnts = append(genMnts,
			mountWrapper{
				Mount:          m,
				DockerImplicit: false,
			})
	}

	for _, mp := range app.MountPoints {
		// there's already an injected mount for this target path, skip
		if _, ok := mnts[mp.Path]; ok {
			continue
		}
		vol, ok := volumes[mp.Name]
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

			dockerImplicit := convertedFromDocker(imageManifest)
			log.Printf("warning: no volume specified for mount point %q, implicitly creating an \"empty\" volume. This volume will be removed when the pod is garbage-collected.", mp.Name)
			if dockerImplicit {
				log.Printf("Docker converted image, initializing implicit volume with data contained at the mount point %q.", mp.Name)
			}

			volumes[uniqName] = emptyVol
			genMnts = append(genMnts,
				mountWrapper{
					Mount: schema.Mount{
						Volume: uniqName,
						Path:   mp.Path,
					},
					DockerImplicit: dockerImplicit,
				})
		} else {
			genMnts = append(genMnts,
				mountWrapper{
					Mount: schema.Mount{
						Volume: vol.Name,
						Path:   mp.Path,
					},
					DockerImplicit: false,
				})
		}
	}

	return genMnts
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
