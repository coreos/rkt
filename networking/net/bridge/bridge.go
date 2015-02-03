// Copyright 2014 CoreOS, Inc.
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
	"fmt"
	"log"
	"net"
	"os"
	"runtime"
	"syscall"

	"github.com/coreos/rocket/Godeps/_workspace/src/github.com/appc/spec/schema/types"
	"github.com/coreos/rocket/Godeps/_workspace/src/github.com/vishvananda/netlink"

	"github.com/coreos/rocket/networking/ipam"
	rktnet "github.com/coreos/rocket/networking/net"
	"github.com/coreos/rocket/networking/util"
)

const defaultBrName = "rkt0"

type Net struct {
	rktnet.Net
	BrName string `json:"brName"`
	IsGW   bool   `json:"isGW"`
}

func init() {
	// this ensures that main runs only on main thread (thread group leader).
	// since namespace ops (unshare, setns) are done for a single thread, we
	// must ensure that the goroutine does not jump from OS thread to thread
	runtime.LockOSThread()
}

func ensureBridgeAddr(br *netlink.Bridge, ipn *net.IPNet) error {
	addrs, err := netlink.AddrList(br, syscall.AF_INET)
	if err != nil && err != syscall.ENOENT {
		return fmt.Errorf("could not get list of IP addresses: %v", err)
	}

	// if there're no addresses on the bridge, it's ok -- we'll add one
	if len(addrs) > 0 {
		ipnStr := ipn.String()
		for _, a := range addrs {
			// string comp is actually easiest for doing IPNet comps
			if a.IPNet.String() == ipnStr {
				return nil
			}
		}
		return fmt.Errorf("%q already has an IP address different from %v", br.Name, ipn.String())
	}

	addr := &netlink.Addr{IPNet: ipn, Label: ""}
	if err := netlink.AddrAdd(br, addr); err != nil {
		return fmt.Errorf("could not add IP address to %q: %v", br.Name, err)
	}
	return nil
}

func bridgeByName(name string) (*netlink.Bridge, error) {
	l, err := netlink.LinkByName(name)
	if err != nil {
		return nil, fmt.Errorf("could not lookup %q: %v", name, err)
	}
	br, ok := l.(*netlink.Bridge)
	if !ok {
		return nil, fmt.Errorf("%q already exists but is not a bridge", name)
	}
	return br, nil
}

func ensureBridge(brName string, ipn *net.IPNet) (*netlink.Bridge, error) {
	br := &netlink.Bridge{
		LinkAttrs: netlink.LinkAttrs{
			Name: brName,
		},
	}

	if err := netlink.LinkAdd(br); err != nil {
		if err != syscall.EEXIST {
			return nil, fmt.Errorf("could not add %q: %v", brName, err)
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

	if ipn != nil {
		return br, ensureBridgeAddr(br, ipn)
	}

	return br, nil
}

func setupVeth(contID types.UUID, netns string, br *netlink.Bridge, ifName string, ipConf *ipam.IPConfig) error {
	var hostVethName string

	err := util.WithNetNSPath(netns, func(hostNS *os.File) error {
		// create the veth pair in the container and move host end into host netns
		hostVeth, _, err := util.SetupVeth(contID.String(), ifName, nil, hostNS)
		if err != nil {
			return err
		}

		if err = ipam.ApplyIPConfig(ifName, ipConf); err != nil {
			return err
		}

		hostVethName = hostVeth.Attrs().Name
		return nil
	})
	if err != nil {
		return err
	}

	// need to lookup hostVeth again as its index has changed during ns move
	hostVeth, err := netlink.LinkByName(hostVethName)
	if err != nil {
		return fmt.Errorf("failed to lookup %q: %v", hostVethName, err)
	}

	// connect host veth end to the bridge
	if err = netlink.LinkSetMaster(hostVeth, br); err != nil {
		return fmt.Errorf("failed to connect %q to bridge %v: %v", hostVethName, br.Attrs().Name, err)
	}

	return nil
}

func calcGatewayIP(ipn *net.IPNet) net.IP {
	nid := ipn.IP.Mask(ipn.Mask)
	return util.NextIP(nid)
}

func setupBridge(n *Net, ipamConf *ipam.IPConfig) (*netlink.Bridge, error) {
	var gwn *net.IPNet
	if n.IsGW {
		gw := net.IP{}
		if ipamConf.Gateway != nil {
			gw = ipamConf.Gateway
		} else {
			gw = calcGatewayIP(ipamConf.IP)
		}

		gwn = &net.IPNet{
			IP:   gw,
			Mask: ipamConf.IP.Mask,
		}
	}

	// create bridge if necessary
	br, err := ensureBridge(n.BrName, gwn)
	if err != nil {
		return nil, fmt.Errorf("failed to create bridge %q: %v", n.BrName, err)
	}

	return br, nil
}

func cmdAdd(contID, netns, netConf, ifName string) error {
	cid, err := types.NewUUID(contID)
	if err != nil {
		return fmt.Errorf("error parsing ContainerID: %v", err)
	}

	n := &Net{
		BrName: defaultBrName,
	}
	if err := rktnet.LoadNet(netConf, n); err != nil {
		return fmt.Errorf("failed to load %q: %v", netConf, err)
	}

	// run the IPAM plugin and get back the config to apply
	ipConf, err := ipam.ExecPlugin(n.Net.IPAM.Type)
	if err != nil {
		return err
	}

	br, err := setupBridge(n, ipConf)
	if err != nil {
		return err
	}

	if err = setupVeth(*cid, netns, br, ifName, ipConf); err != nil {
		return err
	}

	return rktnet.PrintIfConfig(&rktnet.IfConfig{
		IP: ipConf.IP.IP,
	})
}

func cmdDel(contID, netns, netConf, ifName string) error {
	return util.WithNetNSPath(netns, func(hostNS *os.File) error {
		return util.DelLinkByName(ifName)
	})
}

func main() {
	var err error

	cmd := os.Getenv("RKT_NETPLUGIN_COMMAND")
	contID := os.Getenv("RKT_NETPLUGIN_CONTID")
	netns := os.Getenv("RKT_NETPLUGIN_NETNS")
	ifName := os.Getenv("RKT_NETPLUGIN_IFNAME")
	netConf := os.Getenv("RKT_NETPLUGIN_NETCONF")

	if cmd == "" || contID == "" || netns == "" || ifName == "" || netConf == "" {
		log.Printf("Required env variable missing")
		log.Print("Env: ", os.Environ())
		os.Exit(1)
	}

	switch cmd {
	case "ADD":
		err = cmdAdd(contID, netns, netConf, ifName)

	case "DEL":
		err = cmdDel(contID, netns, netConf, ifName)

	default:
		log.Printf("Unknown RKT_NETPLUGIN_COMMAND: %v", cmd)
		os.Exit(1)
	}

	if err != nil {
		log.Printf("%v: %v", cmd, err)
		os.Exit(1)
	}
}
