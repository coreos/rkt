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

package types

import (
	"fmt"
	"net"

	cnitypes "github.com/appc/cni/pkg/types"
	"github.com/coreos/rkt/networking/netinfo"
	"github.com/vishvananda/netlink"
)

// NetConf local struct extends cnitypes.NetConf with information about masquerading
// similar to CNI plugins
type NetConf struct {
	cnitypes.NetConf
	IPMasq bool `json:"ipMasq"`
	MTU    int  `json:"mtu"`
}

type ActiveNet struct {
	ConfBytes []byte
	Conf      *NetConf
	Runtime   *netinfo.NetInfo
}

// The following methods implement behavior of netDescriber by ActiveNet
// (behavior required by stage1/init/kvm package and its kernel parameters configuration)

func (an *ActiveNet) HostIP() net.IP {
	return an.Runtime.HostIP
}
func (an *ActiveNet) GuestIP() net.IP {
	return an.Runtime.IP
}
func (an *ActiveNet) IfName() string {
	if an.Conf.Type == "macvlan" {
		// macvtap device passed as parameter to lkvm binary have different
		// kind of name, path to /dev/tapN made with N as link index
		link, err := netlink.LinkByName(an.Runtime.IfName)
		if err != nil {
			// TODO: stderr.PrintE(fmt.Sprintf("cannot get interface '%v'", an.Runtime.IfName), err)
			return ""
		}
		return fmt.Sprintf("/dev/tap%d", link.Attrs().Index)
	}
	return an.Runtime.IfName
}
func (an *ActiveNet) Mask() net.IP {
	return an.Runtime.Mask
}
func (an *ActiveNet) Name() string {
	return an.Conf.Name
}
func (an *ActiveNet) IPMasq() bool {
	return an.Conf.IPMasq
}
func (an *ActiveNet) Gateway() net.IP {
	return an.Runtime.IP4.Gateway
}
func (an *ActiveNet) Routes() []cnitypes.Route {
	return an.Runtime.IP4.Routes
}
