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

package vm

import (
	"bufio"
	"crypto/sha512"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/appc/spec/schema/types"
	"github.com/containernetworking/cni/pkg/ip"
	cnitypes "github.com/containernetworking/cni/pkg/types"
	cniutils "github.com/containernetworking/cni/pkg/utils"
	cnisysctl "github.com/containernetworking/cni/pkg/utils/sysctl"

	"github.com/coreos/go-iptables/iptables"
	"github.com/hashicorp/errwrap"
	"github.com/vishvananda/netlink"

	"github.com/rkt/rkt/networking"
	"github.com/rkt/rkt/networking/tuntap"
	"github.com/rkt/rkt/pkg/log"
)

const (
	defaultBrName     = "kvm-cni0"
	defaultSubnetFile = "/run/flannel/subnet.env"
	defaultMTU        = 1500
)

var stderr *log.Logger

// Kind of dirty; we need to number vtaps, so just use a sequential index
var vtapIndex = 0

type PtpNetConf struct {
	cnitypes.NetConf

	IPMasq bool `json:"ipMasq"`
}

type BridgeNetConf struct {
	cnitypes.NetConf

	IPMasq bool   `json:"ipMasq"`
	MTU    int    `json:"mtu"`
	BrName string `json:"bridge"`
	IsGw   bool   `json:"isGateway"`
}

type MacVTapNetConf struct {
	cnitypes.NetConf
	IPMasq bool   `json:"ipMasq"`
	MTU    int    `json:"mtu"`
	Master string `json:"master"`
	Mode   string `json:"mode"`
}

type FlannelNetConf struct {
	cnitypes.NetConf

	IPMasq bool `json:"ipMasq"`
	MTU    int  `json:"mtu"`

	SubnetFile string                 `json:"subnetFile"`
	Delegate   map[string]interface{} `json:"delegate"`
}

// setupTapDevice creates persistent tap device
// and returns a newly created netlink.Link structure
func setupTapDevice(podID types.UUID) (netlink.Link, error) {
	// network device names are limited to 16 characters
	// the suffix %d will be replaced by the kernel with a suitable number
	nameTemplate := fmt.Sprintf("rkt-%s-tap%%d", podID.String()[0:4])
	ifName, err := tuntap.CreatePersistentIface(nameTemplate, tuntap.Tap)
	if err != nil {
		return nil, errwrap.Wrap(errors.New("tuntap persist"), err)
	}

	link, err := netlink.LinkByName(ifName)
	if err != nil {
		return nil, errwrap.Wrap(fmt.Errorf("cannot find link %q", ifName), err)
	}

	if err := netlink.LinkSetUp(link); err != nil {
		return nil, errwrap.Wrap(fmt.Errorf("cannot set link up %q", ifName), err)
	}
	return link, nil
}

const (
	IPv4InterfaceArpProxySysctlTemplate = "net.ipv4.conf.%s.proxy_arp"
)

// setupTapDevice creates persistent macvtap device
// and returns a newly created netlink.Link structure
// using part of pod hash and interface number in interface name
func setupMacVTapDevice(podID types.UUID, config MacVTapNetConf) (netlink.Link, error) {
	master, err := netlink.LinkByName(config.Master)
	if err != nil {
		return nil, errwrap.Wrap(fmt.Errorf("cannot find master device '%v'", config.Master), err)
	}
	var mode netlink.MacvlanMode
	switch config.Mode {
	// if not set - defaults to bridge mode as in:
	// https://github.com/rkt/rkt/blob/master/Documentation/networking.md#macvlan
	case "", "bridge":
		mode = netlink.MACVLAN_MODE_BRIDGE
	case "private":
		mode = netlink.MACVLAN_MODE_PRIVATE
	case "vepa":
		mode = netlink.MACVLAN_MODE_VEPA
	case "passthru":
		mode = netlink.MACVLAN_MODE_PASSTHRU
	default:
		return nil, fmt.Errorf("unsupported macvtap mode: %v", config.Mode)
	}
	mtu := master.Attrs().MTU
	if config.MTU != 0 {
		mtu = config.MTU
	}
	interfaceName := fmt.Sprintf("rkt-%s-vtap%d", podID.String()[0:4], vtapIndex)
	vtapIndex += 1
	link := &netlink.Macvtap{
		Macvlan: netlink.Macvlan{
			LinkAttrs: netlink.LinkAttrs{
				Name:        interfaceName,
				MTU:         mtu,
				ParentIndex: master.Attrs().Index,
			},
			Mode: mode,
		},
	}

	if err := netlink.LinkAdd(link); err != nil {
		return nil, errwrap.Wrap(errors.New("cannot create macvtap interface"), err)
	}

	// TODO: duplicate following lines for ipv6 support, when it will be added in other places
	ipv4SysctlValueName := fmt.Sprintf(IPv4InterfaceArpProxySysctlTemplate, interfaceName)
	if _, err := cnisysctl.Sysctl(ipv4SysctlValueName, "1"); err != nil {
		// remove the newly added link and ignore errors, because we already are in a failed state
		_ = netlink.LinkDel(link)
		return nil, errwrap.Wrap(fmt.Errorf("failed to set proxy_arp on newly added interface %q", interfaceName), err)
	}

	if err := netlink.LinkSetUp(link); err != nil {
		// remove the newly added link and ignore errors, because we already are in a failed state
		_ = netlink.LinkDel(link)
		return nil, errwrap.Wrap(errors.New("cannot set up macvtap interface"), err)
	}
	return link, nil
}

// ExecNetIPAM executes only the IPAM portion of a network configuration.
func ExecNetIPAM(network *networking.Networking, n *networking.ActiveNet, ifName string) error {
	if err := ip.EnableIP4Forward(); err != nil {
		return errwrap.Wrap(errors.New("failed to enable forwarding"), err)
	}

	// patch plugin type only for single IPAM run time, then revert this change
	original_type := n.Conf.Type
	n.Conf.Type = n.Conf.IPAM.Type
	output, err := network.ExecNetPlugin("ADD", n, ifName)
	n.Conf.Type = original_type
	if err != nil {
		return errwrap.Wrap(fmt.Errorf("problem executing network plugin %q (%q)", n.Conf.IPAM.Type, ifName), err)
	}

	result := cnitypes.Result{}
	if err = json.Unmarshal(output, &result); err != nil {
		return errwrap.Wrap(fmt.Errorf("error parsing %q result", n.Conf.Name), err)
	}

	if result.IP4 == nil {
		return fmt.Errorf("net-plugin returned no IPv4 configuration")
	}

	n.Runtime.MergeCNIResult(result)

	return nil
}

func ensureHasAddr(link netlink.Link, ipn *net.IPNet) error {
	addrs, err := netlink.AddrList(link, syscall.AF_INET)
	if err != nil && err != syscall.ENOENT {
		return errwrap.Wrap(errors.New("could not get list of IP addresses"), err)
	}

	// if there're no addresses on the interface, it's ok -- we'll add one
	if len(addrs) > 0 {
		ipnStr := ipn.String()
		for _, a := range addrs {
			// string comp is actually easiest for doing IPNet comps
			if a.IPNet.String() == ipnStr {
				return nil
			}
		}
		return fmt.Errorf("%q already has an IP address different from %v", link.Attrs().Name, ipn.String())
	}

	addr := &netlink.Addr{IPNet: ipn, Label: link.Attrs().Name}
	if err := netlink.AddrAdd(link, addr); err != nil {
		return errwrap.Wrap(fmt.Errorf("could not add IP address to %q", link.Attrs().Name), err)
	}
	return nil
}

func bridgeByName(name string) (*netlink.Bridge, error) {
	l, err := netlink.LinkByName(name)
	if err != nil {
		return nil, errwrap.Wrap(fmt.Errorf("could not lookup %q", name), err)
	}
	br, ok := l.(*netlink.Bridge)
	if !ok {
		return nil, fmt.Errorf("%q already exists but is not a bridge", name)
	}
	return br, nil
}

func EnsureBridgeIsUp(brName string, mtu int) (*netlink.Bridge, error) {
	br := &netlink.Bridge{
		LinkAttrs: netlink.LinkAttrs{
			Name: brName,
			MTU:  mtu,
		},
	}

	if err := netlink.LinkAdd(br); err != nil {
		if err != syscall.EEXIST {
			return nil, errwrap.Wrap(fmt.Errorf("could not add %q", brName), err)
		}

		// it's ok if the device already exists as long as config is similar
		br, err = bridgeByName(brName)
		if err != nil {
			return nil, err
		}
	}

	if err := netlink.LinkSetUp(br); err != nil {
		return nil, err
	}

	return br, nil
}

func addRoute(link netlink.Link, podIP net.IP) error {
	route := netlink.Route{
		LinkIndex: link.Attrs().Index,
		Scope:     netlink.SCOPE_LINK,
		Dst: &net.IPNet{
			IP:   podIP,
			Mask: net.IPv4Mask(0xff, 0xff, 0xff, 0xff),
		},
	}
	return netlink.RouteAdd(&route)
}

func removeAllRoutesOnLink(link netlink.Link) error {
	routes, err := netlink.RouteList(link, netlink.FAMILY_V4)
	if err != nil {
		return errwrap.Wrap(fmt.Errorf("cannot list routes on link %q", link.Attrs().Name), err)
	}

	for _, route := range routes {
		if err := netlink.RouteDel(&route); err != nil {
			return errwrap.Wrap(fmt.Errorf("error in time of route removal for route %q", route), err)
		}
	}

	return nil
}

func getChainName(podUUIDString, confName string) string {
	h := sha512.Sum512([]byte(podUUIDString))
	return fmt.Sprintf("CNI-%s-%x", confName, h[:8])
}

func loadFlannelNetConf(bytes []byte) (*FlannelNetConf, error) {
	n := &FlannelNetConf{
		SubnetFile: defaultSubnetFile,
	}
	if err := json.Unmarshal(bytes, n); err != nil {
		return nil, errwrap.Wrap(errors.New("failed to load netconf"), err)
	}
	return n, nil
}

type subnetEnv struct {
	nw     *net.IPNet
	sn     *net.IPNet
	mtu    int
	ipmasq bool
}

func loadFlannelSubnetEnv(fn string) (*subnetEnv, error) {
	f, err := os.Open(fn)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	se := &subnetEnv{}

	s := bufio.NewScanner(f)
	for s.Scan() {
		parts := strings.SplitN(s.Text(), "=", 2)
		switch parts[0] {
		case "FLANNEL_NETWORK":
			_, se.nw, err = net.ParseCIDR(parts[1])
			if err != nil {
				return nil, err
			}

		case "FLANNEL_SUBNET":
			_, se.sn, err = net.ParseCIDR(parts[1])
			if err != nil {
				return nil, err
			}

		case "FLANNEL_MTU":
			mtu, err := strconv.ParseUint(parts[1], 10, 32)
			if err != nil {
				return nil, err
			}
			se.mtu = int(mtu)

		case "FLANNEL_IPMASQ":
			se.ipmasq = parts[1] == "true"
		}
	}
	if err := s.Err(); err != nil {
		return nil, err
	}

	return se, nil
}

func hasKey(m map[string]interface{}, k string) bool {
	_, ok := m[k]
	return ok
}

func isString(i interface{}) bool {
	_, ok := i.(string)
	return ok
}

func kvmTransformFlannelNetwork(anet *networking.ActiveNet) error {
	n, err := loadFlannelNetConf(anet.ConfBytes)
	if err != nil {
		return err
	}

	fenv, err := loadFlannelSubnetEnv(n.SubnetFile)
	if err != nil {
		return err
	}

	if n.Delegate == nil {
		n.Delegate = make(map[string]interface{})
	} else {
		if hasKey(n.Delegate, "type") && !isString(n.Delegate["type"]) {
			return fmt.Errorf("'delegate' dictionary, if present, must have (string) 'type' field")
		}
		if hasKey(n.Delegate, "name") {
			return fmt.Errorf("'delegate' dictionary must not have 'name' field, it'll be set by flannel")
		}
		if hasKey(n.Delegate, "ipam") {
			return fmt.Errorf("'delegate' dictionary must not have 'ipam' field, it'll be set by flannel")
		}
	}

	n.Delegate["name"] = n.Name

	if !hasKey(n.Delegate, "type") {
		n.Delegate["type"] = "bridge"
	}

	if !hasKey(n.Delegate, "isDefaultGateway") {
		n.Delegate["isDefaultGateway"] = false
	}

	if !hasKey(n.Delegate, "ipMasq") {
		// if flannel is not doing ipmasq, we should
		ipmasq := !fenv.ipmasq
		n.Delegate["ipMasq"] = ipmasq
	}

	if !hasKey(n.Delegate, "mtu") {
		mtu := fenv.mtu
		n.Delegate["mtu"] = mtu
	}

	if n.Delegate["type"].(string) == "bridge" {
		if !hasKey(n.Delegate, "isGateway") {
			n.Delegate["isGateway"] = true
		}
	}

	n.Delegate["ipam"] = map[string]interface{}{
		"type":   "host-local",
		"subnet": fenv.sn.String(),
		"routes": []cnitypes.Route{
			{
				Dst: *fenv.nw,
			},
		},
	}

	bytes, err := json.Marshal(n.Delegate)
	if err != nil {
		return errwrap.Wrap(errors.New("error in marshaling generated network settings"), err)
	}

	anet.Runtime.IP4 = &cnitypes.IPConfig{}
	*anet = networking.ActiveNet{
		ConfBytes: bytes,
		Conf:      &cnitypes.NetConf{},
		Runtime:   anet.Runtime,
	}
	anet.Conf.Name = n.Name
	anet.Conf.Type = n.Delegate["type"].(string)
	anet.Conf.IPAM.Type = "host-local"
	return nil
}

// kvmSetup prepare new Networking to be used in kvm environment based on tuntap pair interfaces
// to allow communication with virtual machine created by lkvm tool
func KvmSetup(n *networking.Networking, debug bool) {
	stderr = log.New(os.Stderr, "networking/vm", debug)

	n.CreateNS = false
	n.AddNet = KvmAddNetwork
	n.DelNet = KvmDelNetwork
}

func KvmAddNetwork(n *networking.Networking, anet *networking.ActiveNet) error {
	_, defaultNet, _ := net.ParseCIDR("0.0.0.0/0")

	if anet.Conf.Type == "flannel" {
		if err := kvmTransformFlannelNetwork(anet); err != nil {
			return errwrap.Wrap(errors.New("cannot transform flannel network into basic network"), err)
		}
	}

	enableIpmasq := false

	switch anet.Conf.Type {
	case "ptp":
		config := PtpNetConf{}
		if err := json.Unmarshal(anet.ConfBytes, &config); err != nil {
			return errwrap.Wrap(fmt.Errorf("error parsing %q result", anet.Conf.Name), err)
		}

		enableIpmasq = config.IPMasq

		link, err := setupTapDevice(n.Pod.UUID)
		if err != nil {
			return err
		}
		ifName := link.Attrs().Name
		anet.Runtime.IfName = ifName

		err = ExecNetIPAM(n, anet, ifName)
		if err != nil {
			return err
		}

		// add address to host tap device
		err = ensureHasAddr(
			link,
			&net.IPNet{
				IP:   anet.Runtime.IP4.Gateway,
				Mask: net.IPMask(anet.Runtime.Mask),
			},
		)
		if err != nil {
			return errwrap.Wrap(fmt.Errorf("cannot add address to host tap device %q", ifName), err)
		}

		if err := removeAllRoutesOnLink(link); err != nil {
			return errwrap.Wrap(fmt.Errorf("cannot remove route on host tap device %q", ifName), err)
		}

		if err := addRoute(link, anet.Runtime.IP); err != nil {
			return errwrap.Wrap(errors.New("cannot add on host direct route to pod"), err)
		}

	case "bridge":
		config := BridgeNetConf{
			BrName: defaultBrName,
			MTU:    defaultMTU,
		}
		if err := json.Unmarshal(anet.ConfBytes, &config); err != nil {
			return errwrap.Wrap(fmt.Errorf("error parsing %q result", anet.Conf.Name), err)
		}
		enableIpmasq = config.IPMasq

		br, err := EnsureBridgeIsUp(config.BrName, config.MTU)
		if err != nil {
			return errwrap.Wrap(errors.New("error in time of bridge setup"), err)
		}
		link, err := setupTapDevice(n.Pod.UUID)
		if err != nil {
			return errwrap.Wrap(errors.New("can not setup tap device"), err)
		}
		err = netlink.LinkSetMaster(link, br)
		if err != nil {
			rErr := tuntap.RemovePersistentIface(anet.Runtime.IfName, tuntap.Tap)
			if rErr != nil {
				stderr.PrintE("warning: could not cleanup tap interface", rErr)
			}
			return errwrap.Wrap(errors.New("can not add tap interface to bridge"), err)
		}

		ifName := link.Attrs().Name
		anet.Runtime.IfName = ifName

		err = ExecNetIPAM(n, anet, ifName)
		if err != nil {
			return err
		}

		if config.IsGw {
			anet.Runtime.IP4.Routes = append(
				anet.Runtime.IP4.Routes,
				cnitypes.Route{Dst: *defaultNet, GW: anet.Runtime.IP4.Gateway},
			)
			config.IsGw = true
		}

		if config.IsGw {
			err = ensureHasAddr(
				br,
				&net.IPNet{
					IP:   anet.Runtime.IP4.Gateway,
					Mask: net.IPMask(anet.Runtime.Mask),
				},
			)

			if err != nil {
				return errwrap.Wrap(fmt.Errorf("cannot add address to host bridge device %q", br.Name), err)
			}
		}

	case "macvlan":
		config := MacVTapNetConf{}
		if err := json.Unmarshal(anet.ConfBytes, &config); err != nil {
			return errwrap.Wrap(fmt.Errorf("error parsing %q result", anet.Conf.Name), err)
		}
		enableIpmasq = config.IPMasq

		link, err := setupMacVTapDevice(n.Pod.UUID, config)
		if err != nil {
			return err
		}
		ifName := link.Attrs().Name
		anet.Runtime.IfName = ifName

		err = ExecNetIPAM(n, anet, ifName)
		if err != nil {
			return err
		}

	default:
		return fmt.Errorf("network %q has unsupported type: %q", anet.Conf.Name, anet.Conf.Type)
	}

	if enableIpmasq {
		chain := cniutils.FormatChainName(anet.Conf.Name, n.Pod.UUID.String())
		comment := cniutils.FormatComment(anet.Conf.Name, n.Pod.UUID.String())
		if err := ip.SetupIPMasq(&net.IPNet{
			IP:   anet.Runtime.IP,
			Mask: net.IPMask(anet.Runtime.Mask),
		}, chain, comment); err != nil {
			return err
		}
	}
	return nil
}

// KvmDelNetwork deletes a network
func KvmDelNetwork(n *networking.Networking, an *networking.ActiveNet) error {
	stderr.Printf("KvmDelNetwork: delete network %s", an.Conf.Name)
	if an.Conf.Type == "flannel" {
		if err := kvmTransformFlannelNetwork(an); err != nil {
			return fmt.Errorf("error transforming flannel network: %v", err)
		}
	}

	switch an.Conf.Type {
	case "ptp", "bridge":
		// remove tuntap interface
		tuntap.RemovePersistentIface(an.Runtime.IfName, tuntap.Tap)

	case "macvlan":
		link, err := netlink.LinkByName(an.Runtime.IfName)
		if err != nil {
			return fmt.Errorf("cannot find link `%v`: %v", an.Runtime.IfName, err)
		} else {
			err := netlink.LinkDel(link)
			if err != nil {
				return fmt.Errorf("cannot remove link `%v`: %v", an.Runtime.IfName, err)
			}
		}

	default:
		return fmt.Errorf("unsupported network type: %q", an.Conf.Type)
	}

	// ugly hack again to directly call IPAM plugin to release IP
	an.Conf.Type = an.Conf.IPAM.Type

	_, err := n.ExecNetPlugin("DEL", an, an.Runtime.IfName)
	if err != nil {
		stderr.PrintE("error executing network plugin", err)
	}

	// remove masquerading if it was prepared
	chainName := cniutils.FormatChainName(an.Conf.Name, n.Pod.UUID.String())
	ipt, err := iptables.New()
	if err != nil {
		return err
	}

	chains, err := ipt.ListChains("nat")
	if err != nil {
		return err
	}

	for _, ch := range chains {
		stderr.Printf("%s == %s ?", chainName, ch)
		if ch == chainName {
			comment := cniutils.FormatComment(an.Conf.Name, n.Pod.UUID.String())
			err := ip.TeardownIPMasq(&net.IPNet{
				IP:   an.Runtime.IP,
				Mask: net.IPMask(an.Runtime.Mask),
			}, chainName, comment)
			if err != nil {
				stderr.PrintE("error on removing masquerading", err)
			}
			break
		}
	}

	return nil
}

func IfName(an networking.ActiveNet) string {
	if an.Conf.Type == "macvlan" {
		// macvtap device passed as parameter to lkvm binary have different
		// kind of name, path to /dev/tapN made with N as link index
		link, err := netlink.LinkByName(an.Runtime.IfName)
		if err != nil {
			stderr.PrintE(fmt.Sprintf("cannot get interface '%v'", an.Runtime.IfName), err)
			return ""
		}
		return fmt.Sprintf("/dev/tap%d", link.Attrs().Index)
	}
	return an.Runtime.IfName
}
