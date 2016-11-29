// Copyright 2016 The rkt Authors
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

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/coreos/go-systemd/util"
	"github.com/coreos/rkt/common"
	"github.com/coreos/rkt/common/cgroup/v1"
	"github.com/coreos/rkt/common/cgroup/v2"
	"github.com/hashicorp/errwrap"
)

func areHostV1CgroupsMounted(enabledV1Cgroups map[int][]string) bool {
	controllers := v1.GetControllerDirs(enabledV1Cgroups)
	for _, c := range controllers {
		if !v1.IsControllerMounted(c) {
			return false
		}
	}

	return true
}

// mountHostV1Cgroups mounts the host v1 cgroup hierarchy as required by
// systemd-nspawn. We need this because some distributions don't have the
// "name=systemd" cgroup or don't mount the cgroup controllers in
// "/sys/fs/cgroup", and systemd-nspawn needs this. Since this is mounted
// inside the rkt mount namespace, it doesn't affect the host.
func mountHostV1Cgroups(enabledCgroups map[int][]string) error {
	systemdControllerPath := "/sys/fs/cgroup/systemd"
	if !areHostV1CgroupsMounted(enabledCgroups) {
		mountContext := os.Getenv(common.EnvSELinuxMountContext)
		if err := v1.CreateCgroups("/", enabledCgroups, mountContext); err != nil {
			return errwrap.Wrap(errors.New("error creating host cgroups"), err)
		}
	}

	if !v1.IsControllerMounted("systemd") {
		if err := os.MkdirAll(systemdControllerPath, 0700); err != nil {
			return err
		}
		if err := syscall.Mount("cgroup", systemdControllerPath, "cgroup", 0, "none,name=systemd"); err != nil {
			return errwrap.Wrap(fmt.Errorf("error mounting name=systemd hierarchy on %q", systemdControllerPath), err)
		}
	}

	return nil
}

// mountContainerV1Cgroups mounts the cgroup controllers hierarchy in the container's
// namespace read-only, leaving the needed knobs in the subcgroup for each-app
// read-write so systemd inside stage1 can apply isolators to them
func mountContainerV1Cgroups(s1Root string, enabledCgroups map[int][]string, subcgroup string, serviceNames []string) error {
	mountContext := os.Getenv(common.EnvSELinuxMountContext)
	if err := v1.CreateCgroups(s1Root, enabledCgroups, mountContext); err != nil {
		return errwrap.Wrap(errors.New("error creating container cgroups"), err)
	}
	if err := v1.RemountCgroupsRO(s1Root, enabledCgroups, subcgroup, serviceNames); err != nil {
		return errwrap.Wrap(errors.New("error restricting container cgroups"), err)
	}

	return nil
}

func getContainerSubCgroup(machineID string, canMachinedRegister, unified bool) (string, error) {
	var subcgroup string
	fromUnit, err := util.RunningFromSystemService()
	if err != nil {
		return "", errwrap.Wrap(errors.New("could not determine if we're running from a unit file"), err)
	}
	if fromUnit {
		slice, err := util.GetRunningSlice()
		if err != nil {
			return "", errwrap.Wrap(errors.New("could not get slice name"), err)
		}
		slicePath, err := common.SliceToPath(slice)
		if err != nil {
			return "", errwrap.Wrap(errors.New("could not convert slice name to path"), err)
		}
		unit, err := util.CurrentUnitName()
		if err != nil {
			return "", errwrap.Wrap(errors.New("could not get unit name"), err)
		}
		subcgroup = filepath.Join(slicePath, unit)

		if unified {
			subcgroup = filepath.Join(subcgroup, "payload")
		}
	} else {
		escapedmID := strings.Replace(machineID, "-", "\\x2d", -1)
		machineDir := "machine-" + escapedmID + ".scope"
		if canMachinedRegister {
			// we are not in the final cgroup yet: systemd-nspawn will move us
			// to the correct cgroup later during registration so we can't
			// look it up in /proc/self/cgroup
			subcgroup = filepath.Join("machine.slice", machineDir)
		} else {
			if unified {
				var err error
				subcgroup, err = v2.GetOwnCgroupPath()
				if err != nil {
					return "", errwrap.Wrap(errors.New("could not get own v2 cgroup path"), err)
				}
			} else {
				// when registration is disabled the container will be directly
				// under the current cgroup so we can look it up in /proc/self/cgroup
				ownV1CgroupPath, err := v1.GetOwnCgroupPath("name=systemd")
				if err != nil {
					return "", errwrap.Wrap(errors.New("could not get own v1 cgroup path"), err)
				}
				// systemd-nspawn won't work if we are in the root cgroup. In addition,
				// we want all rkt instances to be in distinct cgroups. Create a
				// subcgroup and add ourselves to it.
				subcgroup = filepath.Join(ownV1CgroupPath, machineDir)
				if err := v1.JoinSubcgroup("systemd", subcgroup); err != nil {
					return "", errwrap.Wrap(fmt.Errorf("error joining %s subcgroup", ownV1CgroupPath), err)
				}
			}
		}
	}

	return subcgroup, nil
}
