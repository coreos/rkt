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

package networking

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"strings"
	"syscall"

	"github.com/appc/spec/schema/types"
	"github.com/containernetworking/cni/pkg/ip"
	cnitypes "github.com/containernetworking/cni/pkg/types"
	"github.com/hashicorp/errwrap"
	"github.com/vishvananda/netlink"

	"github.com/rkt/rkt/common"
	commonnet "github.com/rkt/rkt/common/networking"
	"github.com/rkt/rkt/networking/netinfo"
	"github.com/rkt/rkt/pkg/log"
)

const (
	IfNamePattern = "eth%d"
	selfNetNS     = "/proc/self/ns/net"
)

// Networking describes the networking details of a pod.
type Networking struct {
	podEnv

	nets []activeNet
}

// NetConf local struct extends cnitypes.NetConf with information about masquerading
// similar to CNI plugins
type NetConf struct {
	cnitypes.NetConf
	IPMasq           bool `json:"ipMasq"`
	MTU              int  `json:"mtu"`
	IsDefaultGateway bool `json:"isDefaultGateway"`
}

var (
	stderr   *log.Logger
	debuglog bool
)

// vmSetupNetAddressing calls IPAM plugin (with a hack) to reserve an IP to be
// used by newly create tuntap pair
// in result it updates activeNet.runtime configuration
func vmSetupNetAddressing(network *Networking, n activeNet, ifName string) error {
	// TODO: very ugly hack, that go through upper plugin, down to ipam plugin
	if err := ip.EnableIP4Forward(); err != nil {
		return errwrap.Wrap(errors.New("failed to enable forwarding"), err)
	}

	// patch plugin type only for single IPAM run time, then revert this change
	original_type := n.conf.Type
	n.conf.Type = n.conf.IPAM.Type
	output, err := network.execNetPlugin("ADD", &n, ifName)
	n.conf.Type = original_type
	if err != nil {
		return errwrap.Wrap(fmt.Errorf("problem executing network plugin %q (%q)", n.conf.IPAM.Type, ifName), err)
	}

	result := cnitypes.Result{}
	if err = json.Unmarshal(output, &result); err != nil {
		return errwrap.Wrap(fmt.Errorf("error parsing %q result", n.conf.Name), err)
	}

	if result.IP4 == nil {
		return fmt.Errorf("net-plugin returned no IPv4 configuration")
	}

	n.runtime.MergeCNIResult(result)

	return nil
}

func xenSetup(podRoot string, podID types.UUID, fps []commonnet.ForwardedPort, netList common.NetList, localConfig string, noDNS bool) (*Networking, error) {
	network := Networking{
		podEnv: podEnv{
			podRoot:      podRoot,
			podID:        podID,
			netsLoadList: netList,
			localConfig:  localConfig,
		},
	}
	var e error
	network.nets, e = network.loadNets()
	if e != nil {
		return nil, errwrap.Wrap(errors.New("error loading network definitions"), e)
	}

	for _, n := range network.nets {
		if n.conf.Type == "flannel" {
			return nil, errors.New("cannot transform flannel network into basic network")
		}
		n.runtime.IfName = "vif"
		switch n.conf.Type {
		case "ptp":
			err := vmSetupNetAddressing(&network, n, "vif")
			if err != nil {
				return nil, err
			}

		case "bridge":
			config := BridgeNetConf{
				NetConf: NetConf{
					MTU: defaultMTU,
				},
				BrName: defaultBrName,
			}
			if err := json.Unmarshal(n.confBytes, &config); err != nil {
				return nil, errwrap.Wrap(fmt.Errorf("error parsing %q result", n.conf.Name), err)
			}

			_, err := ensureBridgeIsUp(config.BrName, config.MTU)
			if err != nil {
				return nil, errwrap.Wrap(errors.New("error in time of bridge setup"), err)
			}

			n.runtime.IfName = config.BrName
			err2 := vmSetupNetAddressing(&network, n, n.runtime.IfName)
			if err2 != nil {
				return nil, err2
			}

		default:
			return nil, fmt.Errorf("network %q have unsupported type: %q", n.conf.Name, n.conf.Type)
		}
	}

	return &network, nil
}

// Setup creates a new networking namespace and executes network plugins to
// set up networking. It returns in the new pod namespace
func Setup(podRoot string, podID types.UUID, fps []commonnet.ForwardedPort, netList common.NetList, localConfig, flavor string, noDNS, debug bool) (*Networking, error) {

	stderr = log.New(os.Stderr, "networking", debug)
	debuglog = debug

	if flavor == "xen" {
		return xenSetup(podRoot, podID, fps, netList, localConfig, noDNS)
	}

	if flavor == "kvm" {
		return kvmSetup(podRoot, podID, fps, netList, localConfig, noDNS)
	}

	// TODO(jonboulle): currently podRoot is _always_ ".", and behaviour in other
	// circumstances is untested. This should be cleaned up.
	n := Networking{
		podEnv: podEnv{
			podRoot:      podRoot,
			podID:        podID,
			netsLoadList: netList,
			localConfig:  localConfig,
		},
	}

	// Create the network namespace (and save its name in a file)
	err := n.podNSCreate()
	if err != nil {
		return nil, err
	}

	n.nets, err = n.loadNets()
	if err != nil {
		return nil, errwrap.Wrap(errors.New("error loading network definitions"), err)
	}

	if err := n.setupNets(n.nets, noDNS); err != nil {
		return nil, err
	}

	if len(fps) > 0 {
		if err = n.enableDefaultLocalnetRouting(); err != nil {
			return nil, err
		}
		podIP, err := n.GetForwardableNetPodIP()
		if err != nil {
			return nil, err
		}
		if err := n.setupForwarding(); err != nil {
			n.teardownForwarding()
			return nil, err
		}
		if err := n.forwardPorts(fps, podIP); err != nil {
			n.teardownForwarding()
			return nil, err
		}
	}

	// Switch to the podNS
	if err := n.podNS.Set(); err != nil {
		return nil, err
	}

	if err = loUp(); err != nil {
		return nil, err
	}

	return &n, nil
}

// enableDefaultLocalnetRouting enables the route_localnet attribute on the supposedly default network interface.
// This allows setting up loopback NAT so the host can access the pod's forwarded ports on the localhost address.
func (n *Networking) enableDefaultLocalnetRouting() error {
	routeLocalnetFormat := ""

	defaultHostIP, err := n.GetForwardableNetHostIP()
	if err != nil {
		return err
	}

	defaultHostIPstring := defaultHostIP.String()
	switch {
	case strings.Contains(defaultHostIPstring, "."):
		routeLocalnetFormat = "/proc/sys/net/ipv4/conf/%s/route_localnet"
	case strings.Contains(defaultHostIPstring, ":"):
		return fmt.Errorf("unexpected IPv6 Address returned for default host interface: %q", defaultHostIPstring)
	default:
		return fmt.Errorf("unknown type for default Host IP: %q", defaultHostIPstring)
	}

	hostIfaces, err := n.GetIfacesByIP(defaultHostIP)
	if err != nil {
		return err
	}

	for _, hostIface := range hostIfaces {
		routeLocalnetPath := fmt.Sprintf(routeLocalnetFormat, hostIface.Name)
		routeLocalnetValue, err := ioutil.ReadFile(routeLocalnetPath)
		if err != nil {
			return err
		}
		if string(routeLocalnetValue) != "1" {
			routeLocalnetFile, err := os.OpenFile(routeLocalnetPath, os.O_WRONLY, 0)
			if err != nil {
				return err
			}
			defer routeLocalnetFile.Close()

			if _, err = io.WriteString(routeLocalnetFile, "1"); err != nil {
				return err
			}
		}
	}

	return nil
}

// Load creates the Networking object from saved state.
// Assumes the current netns is that of the host.
func Load(podRoot string, podID *types.UUID, localConfig string) (*Networking, error) {
	// the current directory is pod root
	pdirfd, err := syscall.Open(podRoot, syscall.O_RDONLY|syscall.O_DIRECTORY, 0)
	if err != nil {
		return nil, errwrap.Wrap(fmt.Errorf("failed to open pod root directory (%v)", podRoot), err)
	}
	defer syscall.Close(pdirfd)

	nis, err := netinfo.LoadAt(pdirfd)
	if err != nil {
		return nil, err
	}

	var nets []activeNet
	for _, ni := range nis {
		n, err := loadNet(ni.ConfPath)
		if err != nil {
			if !os.IsNotExist(err) {
				stderr.PrintE(fmt.Sprintf("error loading %q; ignoring", ni.ConfPath), err)
			}
			continue
		}

		// make a copy of ni to make it a unique object as it's saved via ptr
		rti := ni
		n.runtime = &rti
		nets = append(nets, *n)
	}

	p := podEnv{
		podRoot:     podRoot,
		podID:       *podID,
		localConfig: localConfig,
	}

	err = p.podNSLoad()
	if err != nil {
		return nil, err
	}

	return &Networking{
		podEnv: p,
		nets:   nets,
	}, nil
}

// GetIfacesByIP searches for and returns the interfaces with the given IP
// Disregards the subnet mask since not every net.IP object contains
// On success it will return the list of found interfaces
func (n *Networking) GetIfacesByIP(ifaceIP net.IP) ([]net.Interface, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	searchAddr := strings.Split(ifaceIP.String(), "/")[0]
	resultInterfaces := make([]net.Interface, 0)

	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			return nil, errwrap.Wrap(fmt.Errorf("cannot get addresses for interface %v", iface.Name), err)
		}

		for _, addr := range addrs {
			currentAddr := strings.Split(addr.String(), "/")[0]
			if searchAddr == currentAddr {
				resultInterfaces = append(resultInterfaces, iface)
				break
			}
		}
	}

	if len(resultInterfaces) == 0 {
		return nil, fmt.Errorf("no interface found with IP %q", ifaceIP)
	}

	return resultInterfaces, nil
}

// Teardown cleans up a produced Networking object.
func (n *Networking) Teardown(flavor string, debug bool) {

	stderr = log.New(os.Stderr, "networking", debug)
	debuglog = debug

	// Teardown everything in reverse order of setup.
	// This should be idempotent -- be tolerant of missing stuff

	if flavor == "kvm" {
		n.kvmTeardown()
		return
	}

	if err := n.teardownForwarding(); err != nil {
		stderr.PrintE("error removing forwarded ports", err)
	}

	err := n.podNSLoad()
	if err != nil {
		stderr.PrintE("error loading podNS", err)
	}

	n.teardownNets(n.nets)
	n.podNSDestroy()
}

// Save writes out the info about active nets
// for "rkt list" and friends to display
func (e *Networking) Save() error {

	if e.podNS != nil {
		if err := e.podNSPathSave(); err != nil {
			return err
		}
	}

	var nis []netinfo.NetInfo
	for _, n := range e.nets {
		nis = append(nis, *n.runtime)
	}

	return netinfo.Save(e.podRoot, nis)
}

// CleanUpGarbage can be called when Load fails, but there may still
// be some garbage lying around. Right now, this deletes the namespace.
func CleanUpGarbage(podRoot string, podID *types.UUID) error {
	p := podEnv{
		podRoot: podRoot,
		podID:   *podID,
	}

	err := p.podNSLoad()
	if err != nil {
		return err
	}
	return p.podNSDestroy()
}

func loUp() error {
	lo, err := netlink.LinkByName("lo")
	if err != nil {
		return errwrap.Wrap(errors.New("failed to lookup lo"), err)
	}

	if err := netlink.LinkSetUp(lo); err != nil {
		return errwrap.Wrap(errors.New("failed to set lo up"), err)
	}

	return nil
}
