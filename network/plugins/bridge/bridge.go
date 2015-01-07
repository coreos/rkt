package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"runtime"
	"syscall"

	"github.com/coreos/rocket/Godeps/_workspace/src/github.com/vishvananda/netlink"

	"github.com/coreos/rocket/network/util"
)

const defaultBrName = "rkt0"

type Net struct {
	util.Net
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

	addr := &netlink.Addr{ipn, ""}
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

func loadConf(path string) (*Net, error) {
	conf, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	n := &Net{
		BrName: defaultBrName,
	}

	err = json.Unmarshal(conf, n)
	if err != nil {
		return nil, err
	}

	return n, nil
}

func parseCIDR(s string) (*net.IPNet, error) {
	ip, ipn, err := net.ParseCIDR(s)
	if err != nil {
		return nil, err
	}

	ipn.IP = ip
	return ipn, nil
}

func cmdAdd(contID, netns, netConf, ifName string) error {
	conf, err := loadConf(netConf)
	if err != nil {
		return fmt.Errorf("Failed to load %q: %v", netConf, err)
	}

	// create bridge if necessary
	var ipn *net.IPNet
	if conf.IsGW && conf.IPAlloc.Type == "static" {
		ipn, err = parseCIDR(conf.IPAlloc.Subnet)
		if err != nil {
			return fmt.Errorf("Error parsing ipAlloc.Subnet: %v", err)
		}
	}
	br, err := ensureBridge(conf.BrName, ipn)
	if err != nil {
		return fmt.Errorf("Failed to create bridge %q: %v", conf.BrName, err)
	}

	// save a handle to current (host) network namespace
	thisNS, err := os.Open("/proc/self/ns/net")
	if err != nil {
		return fmt.Errorf("Failed to open /proc/self/ns/net: %v", err)
	}

	// switch to the container namespace
	contNS, err := os.Open(netns)
	if err != nil {
		return fmt.Errorf("Failed to open %v: %v", netns, err)
	}

	if err = util.SetNS(contNS, syscall.CLONE_NEWNET); err != nil {
		return fmt.Errorf("Error switching to ns %v: %v", netns, err)
	}

	// create the veth pair in the container and move host end into host netns
	hostVeth, _, err := util.SetupVeth(contID, ifName, nil, thisNS)
	if err != nil {
		return fmt.Errorf("Error: %v", err)
	}

	// switch back to host netns and plug the host veth end into the bridge
	if err = util.SetNS(thisNS, syscall.CLONE_NEWNET); err != nil {
		return fmt.Errorf("Error switching to host netns: %v", err)
	}

	// need to lookup hostVeth again as its index has changed during ns move
	hostVeth, err = netlink.LinkByName(hostVeth.Attrs().Name)
	if err != nil {
		return fmt.Errorf("Failed to lookup %q: %v", hostVeth.Attrs().Name, err)
	}

	if err = netlink.LinkSetMaster(hostVeth, br); err != nil {
		return fmt.Errorf("Failed to connect %q to bridge %q: %v", hostVeth.Attrs().Name, conf.BrName, err)
	}

	return nil
}

func cmdDel(contID, netns, netConf, ifName string) error {
	// TODO
	return nil
}

func usage() int {
	fmt.Fprintln(os.Stderr, "USAGE: add|del CONTAINER-ID NETNS NET-CONF IF-NAME")
	return 1
}

func main() {
	if len(os.Args) != 6 {
		os.Exit(usage())
	}

	var err error

	switch os.Args[1] {
	case "add":
		err = cmdAdd(os.Args[2], os.Args[3], os.Args[4], os.Args[5])

	case "del":
		err = cmdDel(os.Args[2], os.Args[3], os.Args[4], os.Args[5])

	default:
		os.Exit(usage())
	}

	if err != nil {
		log.Print(err)
		os.Exit(1)
	}
}
