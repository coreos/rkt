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

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/schema"
	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/schema/types"
)

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

func GenerateMounts(ra *schema.RuntimeApp, volumes map[types.ACName]types.Volume) ([]schema.Mount, error) {
	appName := ra.Name
	id := ra.Image.ID
	app := ra.App

	mnts := make(map[string]schema.Mount)
	for _, m := range ra.Mounts {
		mnts[m.Path] = m
	}

	for _, mp := range app.MountPoints {
		// there's already an injected mount for this target path, skip
		if _, ok := mnts[mp.Path]; ok {
			continue
		}
		vol, ok := volumes[mp.Name]
		if !ok {
			catCmd := fmt.Sprintf("sudo rkt image cat-manifest --pretty-print %s", id.String()[:19])
			volumeCmd := ""
			for _, mp := range app.MountPoints {
				volumeCmd += fmt.Sprintf("--volume %s,kind=host,source=/some/path ", mp.Name)
			}

			return nil, fmt.Errorf("no volume for mountpoint %q:%q in app %q.\n"+
				"You can inspect the volumes with:\n\t%v\n"+
				"App %q requires the following volumes:\n\t%v",
				mp.Name, mp.Path, appName, catCmd, appName, volumeCmd)
		}
		ra.Mounts = append(ra.Mounts, schema.Mount{Volume: vol.Name, Path: mp.Path})
	}

	return ra.Mounts, nil
}
