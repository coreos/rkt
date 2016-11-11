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

//+build linux

package main

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"strconv"
	"syscall"

	"github.com/coreos/rkt/networking/netinfo"
	"github.com/vishvananda/netlink"
)

func findIface(ip net.IP) (*netlink.Link, error) {
	links, err := netlink.LinkList()
	if err != nil {
		return nil, fmt.Errorf("Cannot find links!")
	}

	for _, link := range links {
		addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
		if err != nil {
			return nil, fmt.Errorf("Cannot list links for %s: %v", link.Attrs().Name, err)
		}

		for _, addr := range addrs {
			if bytes.Compare(addr.IP, ip) == 0 {
				err = netlink.AddrDel(link, &addr)
				if err != nil {
					return nil, err
				}
				return &link, nil
			}
		}
	}

	return nil, fmt.Errorf("Cannot find interface with IP address: %v", ip)
}

/*
 * setupBridge: creates new bridge, attaches to it veth device and tap
 */
func setupBridge(brID int, net netinfo.NetInfo, tapLink *netlink.Link, nslink *netlink.Link) error {
	brname := fmt.Sprintf("br%d", brID)
	br := &netlink.Bridge{
		LinkAttrs: netlink.LinkAttrs{
			Name: brname,
		},
	}

	if err := netlink.LinkAdd(br); err != nil {
		return err
	}

	if err := netlink.LinkSetUp(br); err != nil {
		return err
	}

	if err := netlink.LinkSetMaster(*nslink, br); err != nil {
		return err
	}

	if err := netlink.LinkSetMaster(*tapLink, br); err != nil {
		return err
	}

	if err := netlink.LinkSetUp(*tapLink); err != nil {
		return err
	}

	return nil
}

func setupTap(tapName string) (*netlink.Link, error) {
	tapLink, err := netlink.LinkByName(tapName)
	if err != nil {
		la := netlink.NewLinkAttrs()
		la.Name = tapName
		mode := netlink.TUNTAP_MODE_TAP
		flags := netlink.TUNTAP_NO_PI | netlink.TUNTAP_VNET_HDR
		tunDesc := &netlink.Tuntap{la, mode, flags}
		if err := netlink.LinkAdd(tunDesc); err != nil {
			return nil, err
		}

		tapLink, err = netlink.LinkByName(tapName)
		if err != nil {
			return nil, err
		}
	}
	return &tapLink, nil
}

func main() {
	if len(os.Args) != 2 {
		fmt.Printf("Device index not specified!\n")
		os.Exit(1)
	}

	tapID, err := strconv.Atoi(os.Args[1][3:])

	currentDirFd, err := syscall.Open(".", syscall.O_RDONLY|syscall.O_DIRECTORY, 0)
	if err != nil {
		fmt.Printf("Cannot open current dir: %v\n", err)
		os.Exit(2)
	}
	defer syscall.Close(currentDirFd)

	netInfos, err := netinfo.LoadAt(currentDirFd)
	if err != nil {
		fmt.Printf("Cannot load net-info.json: %v\n", err)
		os.Exit(3)
	}

	if len(netInfos) <= tapID {
		fmt.Printf("Missing net-info for current interface: %s\n", os.Args[1])
		os.Exit(4)
	}

	tapLink, err := setupTap(os.Args[1])
	if err != nil {
		fmt.Printf("Cannot setup tap dev: %v\n", err)
		os.Exit(5)
	}

	// find veth iface, remove IP address from it
	nsLink, err := findIface(netInfos[tapID].IP.To4())
	if err != nil {
		fmt.Printf("Cannot find veth interface: %v\n", err)
		os.Exit(6)
	}

	err = setupBridge(tapID, netInfos[tapID], tapLink, nsLink)
	if err != nil {
		fmt.Printf("Cannot setup bridge: %v\n", err)
		os.Exit(7)
	}
}
