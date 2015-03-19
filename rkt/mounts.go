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
	"fmt"
	"net/url"
	"strings"

	"github.com/coreos/rocket/Godeps/_workspace/src/github.com/appc/spec/schema"
	"github.com/coreos/rocket/Godeps/_workspace/src/github.com/appc/spec/schema/types"
)

// volumeList implements the flag.Value interface to contain a set of mappings
// from volume label --> mount path (for host type volumes)
type volumeList []types.Volume

func (vl *volumeList) Set(s string) error {
	vol, err := types.VolumeFromString(s)
	if err != nil {
		return err
	}

	*vl = append(*vl, *vol)
	return nil
}

func (vl *volumeList) String() string {
	var vs []string
	for _, v := range []types.Volume(*vl) {
		vs = append(vs, v.String())
	}
	return strings.Join(vs, " ")
}

// mountsMap implements the flag.Value interface to contain a map of mount mappings
// for associating app-specific mountpoints with volumes:
// --mount app=APPNAME,volume=VOLNAME,target=MNTNAME
type mountsMap map[types.ACName][]schema.Mount

func (ml *mountsMap) Set(s string) error {
	var appName *types.ACName
	mount := schema.Mount{}

	// this is intentionally made similar to types.VolumeFromString()
	m, err := url.ParseQuery(strings.Replace(s, ",", "&", -1))
	if err != nil {
		return err
	}

	for key, val := range m {
		if len(val) > 1 {
			return fmt.Errorf("label %s with multiple values %q", key, val)
		}
		switch key {
		case "app":
			appName, err = types.NewACName(val[0])
			if err != nil {
				return err
			}
		case "volume":
			mv, err := types.NewACName(val[0])
			if err != nil {
				return err
			}
			mount.Volume = *mv
		case "target":
			mp, err := types.NewACName(val[0])
			if err != nil {
				return err
			}
			mount.MountPoint = *mp
		default:
			return fmt.Errorf("unknown mount parameter %q", key)
		}
	}

	// TODO(vc): this seems like quite the contortion, golang why must you hate me.
	if *ml == nil {
		*ml = make(map[types.ACName][]schema.Mount)
	}
	mm := map[types.ACName][]schema.Mount(*ml)

	mm[*appName] = append(mm[*appName], mount)
	return nil
}

func (ml *mountsMap) String() string {
	// TODO(vc): return something meaningful
	return ""
}

// Validate ensures all apps in ml exist in apps
func (ml *mountsMap) ValidateAppNames(apps []*types.ACName) error {
	dict := make(map[types.ACName]struct{})

	for _, app := range apps {
		dict[*app] = struct{}{}
	}

	for app, _ := range map[types.ACName][]schema.Mount(*ml) {
		if _, exists := dict[app]; !exists {
			return fmt.Errorf("app %q doesn't exist", app)
		}
	}
	return nil
}

// GetAppMounts returns the mounts list for the named app
func (ml *mountsMap) GetAppMounts(app *types.ACName) []schema.Mount {
	m, _ := map[types.ACName][]schema.Mount(*ml)[*app]
	return m
}
