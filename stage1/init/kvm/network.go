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

package kvm

import (
	"fmt"

	"github.com/coreos/rkt/networking"
)

// GetKVMNetParams returns additional arguments that need to be passed to kernel and lkvm tool to configure networks properly
// parameters are based on Network configuration extracted from Networking struct
func GetKVMNetArgs(n *networking.Networking) ([]string, []string, error) {
	lkvmArgs := []string{}
	kernelParams := []string{}

	for i, netParams := range n.GetNetworkParameters() {
		// https://www.kernel.org/doc/Documentation/filesystems/nfs/nfsroot.txt
		// ip=<client-ip>:<server-ip>:<gw-ip>:<netmask>:<hostname>:<device>:<autoconf>:<dns0-ip>:<dns1-ip>
		var gw string
		if netParams.IPMasq {
			gw = netParams.HostIP.String()
		}
		kernelParams = append(kernelParams, "ip="+netParams.GuestIP.String()+"::"+gw+":"+netParams.Mask.String()+":"+fmt.Sprintf(networking.IfNamePattern, i)+":::")

		lkvmArgs = append(lkvmArgs, "--network", "mode=tap,tapif="+netParams.IfName+",host_ip="+netParams.HostIP.String()+",guest_ip="+netParams.GuestIP.String())
	}

	return lkvmArgs, kernelParams, nil
}
