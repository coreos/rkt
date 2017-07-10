// Copyright 2017 The rkt Authors
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

package vm

import (
	"fmt"
	"net"

	cnitypes "github.com/containernetworking/cni/pkg/types"
	"github.com/rkt/rkt/networking"
	"github.com/vishvananda/netlink"
)

// GetNetworkDescriptions converts activeNets to netDescribers
func GetNetworkDescriptions(n *networking.Networking) []NetDescriber {
	var nds []NetDescriber
	nets := n.Nets()
	for i := 0; i < len(*nets); i++ {
		nds = append(nds, &nd{&(*nets)[i]})
	}
	return nds
}

// NetDescriber is the interface that describes a network configuration
type NetDescriber interface {
	GuestIP() net.IP
	Mask() net.IP
	IfName() string
	Name() string
	Gateway() net.IP
	Routes() []cnitypes.Route
}

type nd struct {
	net *networking.ActiveNet
}

func (n *nd) HostIP() net.IP {
	return n.net.Runtime.HostIP
}
func (n *nd) GuestIP() net.IP {
	return n.net.Runtime.IP
}
func (n *nd) IfName() string {
	if n.net.Conf.Type == "macvlan" {
		// macvtap device passed as parameter to lkvm binary have different
		// kind of name, path to /dev/tapN made with N as link index
		link, err := netlink.LinkByName(n.net.Runtime.IfName)
		if err != nil {
			stderr.PrintE(fmt.Sprintf("cannot get interface '%v'", n.net.Runtime.IfName), err)
			return ""
		}
		return fmt.Sprintf("/dev/tap%d", link.Attrs().Index)
	}
	return n.net.Runtime.IfName
}
func (n *nd) Mask() net.IP {
	return n.net.Runtime.Mask
}
func (n *nd) Name() string {
	return n.net.Conf.Name
}
func (n *nd) Gateway() net.IP {
	return n.net.Runtime.IP4.Gateway
}
func (n *nd) Routes() []cnitypes.Route {
	return n.net.Runtime.IP4.Routes
}
