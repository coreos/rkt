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
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"strings"
	"syscall"

	"github.com/hashicorp/errwrap"
	"github.com/vishvananda/netlink"

	"github.com/rkt/rkt/networking/netinfo"
	"github.com/rkt/rkt/pkg/log"
	stage1commontypes "github.com/rkt/rkt/stage1/common/types"
)

const (
	IfNamePattern = "eth%d"
	selfNetNS     = "/proc/self/ns/net"
)

var (
	stderr   *log.Logger
	debuglog bool
)

func New(pod *stage1commontypes.Pod, localConfigDir string, debug bool) (*Networking, error) {
	stderr = log.New(os.Stderr, "networking", debug)
	debuglog = debug

	n := Networking{
		Pod:            pod,
		LocalConfigDir: localConfigDir,
		CreateNS:       true,
	}

	var err error
	n.nets, err = n.loadNets()
	if err != nil {
		return nil, errwrap.Wrap(errors.New("error loading network definitions"), err)
	}

	return &n, nil
}

// Setup creates a new networking namespace and executes network plugins to
// set up networking.
// NOTE: It switches to the new pod namespace
func (n *Networking) Setup() error {
	if n.CreateNS {
		// Create the network namespace (and save its name in a file)
		err := n.podNSCreate()
		if err != nil {
			return err
		}
	}

	if err := n.setupNets(); err != nil {
		return err
	}

	if err := n.ForwardPorts(); err != nil {
		return err
	}

	if n.CreateNS {
		// Switch to the podNS
		if err := n.PodNS.Set(); err != nil {
			return err
		}

		if err := loUp(); err != nil {
			return err
		}
	}

	return nil
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
// Assumes the current netns is that of the host, but the current mountns
// is the stage1
func Load(pod *stage1commontypes.Pod, localConfigDir string, debug bool) (*Networking, error) {
	stderr = log.New(os.Stderr, "networking", debug)
	debuglog = debug

	// the current directory is pod root
	pdirfd, err := syscall.Open(pod.Root, syscall.O_RDONLY|syscall.O_DIRECTORY, 0)
	if err != nil {
		return nil, errwrap.Wrap(fmt.Errorf("failed to open pod root directory (%v)", pod.Root), err)
	}
	defer syscall.Close(pdirfd)

	nis, err := netinfo.LoadAt(pdirfd)
	if err != nil {
		return nil, err
	}

	var nets []ActiveNet
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
		n.Runtime = &rti
		nets = append(nets, *n)
	}

	n := Networking{
		Pod:            pod,
		LocalConfigDir: localConfigDir,
		nets:           nets,
	}

	n.PodNS, err = podNSLoad(n.Pod.Root)
	if err != nil {
		return nil, err
	}
	return &n, nil
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
func (n *Networking) Teardown() {
	// Teardown everything in reverse order of setup.
	// This should be idempotent -- be tolerant of missing stuff
	if err := n.TeardownForwarding(); err != nil {
		stderr.PrintE("error removing forwarded ports", err)
	}

	var err error
	if n.CreateNS {
		n.PodNS, err = podNSLoad(n.Pod.Root)
		if err != nil {
			stderr.PrintE("error loading podNS", err)
		}
	}

	n.teardownNets(-1)

	if n.PodNS != nil {
		podNSDestroy(n.PodNS)
	}
}

// Save writes out the info about active nets
// for "rkt list" and friends to display
func (n *Networking) Save() error {

	if n.PodNS != nil {
		if err := n.podNSPathSave(); err != nil {
			return err
		}
	}

	var nis []netinfo.NetInfo
	for _, net := range n.nets {
		nis = append(nis, *net.Runtime)
	}

	return netinfo.Save(n.Pod.Root, nis)
}

// CleanUpGarbage can be called when Load fails, but there may still
// be some garbage lying around. Right now, this deletes the namespace.
func CleanUpGarbage(podRoot string) error {
	netns, err := podNSLoad(podRoot)

	if err != nil {
		return err
	}
	return podNSDestroy(netns)
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

// Nets lets you mutate the list of networks - useful for those stage1s
// that implement their own networking
func (n *Networking) Nets() *[]ActiveNet {
	return &n.nets
}

func (n Networking) NoDNS() bool {
	return n.Pod.ResolvConfMode != "default"
}
