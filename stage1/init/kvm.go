// Copyright 2014 The rkt Authors
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
	"path/filepath"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/schema/types"
	"github.com/coreos/rkt/common"
	"github.com/coreos/rkt/networking"
	"github.com/coreos/rkt/stage1/init/kvm"
)

func (p *Pod) KvmPodToSystemd(n *networking.Networking) error {
	podRoot := common.Stage1RootfsPath(p.Root)

	// networking
	netDescriptions := kvm.GetNetworkDescriptions(n)
	if err := kvm.GenerateNetworkInterfaceUnits(filepath.Join(podRoot, unitsDir), netDescriptions); err != nil {
		return fmt.Errorf("failed to transform networking to units: %v", err)
	}

	// volumes
	// prepare all applications names to become dependency for mount units
	// all host-shared folder has to become available before applications starts
	appNames := []types.ACName{}
	for _, runtimeApp := range p.Manifest.Apps {
		appNames = append(appNames, runtimeApp.Name)
	}
	// mount host volumes through some remote file system e.g. 9p to /mnt/volumeName location
	// order is important here: podToSystemHostMountUnits prepares folders that are checked by each appToSystemdMountUnits later
	if err := kvm.PodToSystemdHostMountUnits(podRoot, p.Manifest.Volumes, appNames, unitsDir); err != nil {
		return fmt.Errorf("failed to transform pod volumes into mount units: %v", err)
	}

	return nil
}
