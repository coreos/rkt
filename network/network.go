package network

import (
	"fmt"
	"net"
	"os"
	"syscall"

	"github.com/appc/spec/schema/types"
	"github.com/coreos/rocket/Godeps/_workspace/src/github.com/vishvananda/netlink"

	"github.com/coreos/rocket/network/util"
)

const ifnamePattern = "eth%d"

func newNetNS() (parentNS, childNS *os.File, err error) {
	defer func() {
		if err != nil {
			if parentNS != nil {
				parentNS.Close()
			}
			if childNS != nil {
				childNS.Close()
			}
		}
	}()

	parentNS, err = os.Open("/proc/self/ns/net")
	if err != nil {
		return
	}

	if err = syscall.Unshare(syscall.CLONE_NEWNET); err != nil {
		return
	}

	childNS, err = os.Open("/proc/self/ns/net")
	if err != nil {
		util.SetNS(parentNS, syscall.CLONE_NEWNET)
		return
	}

	return
}

func addDefaultRoute(gw net.IP, dev netlink.Link) error {
	_, defNet, _ := net.ParseCIDR("0.0.0.0/0")
	return netlink.RouteAdd(&netlink.Route{
		LinkIndex: dev.Attrs().Index,
		Scope:     netlink.SCOPE_UNIVERSE,
		Dst:       defNet,
		Gw:        gw,
	})
}

func loUp() error {
	lo, err := netlink.LinkByName("lo")
	if err != nil {
		return fmt.Errorf("failed to lookup lo: %v", err)
	}

	if err := netlink.LinkSetUp(lo); err != nil {
		return fmt.Errorf("failed to set lo up: %v", err)
	}

	return nil
}

func setContIfAddr(parentNS, contNS *os.File, ifName string, ipn *net.IPNet) error {
	if err := util.SetNS(contNS, syscall.CLONE_NEWNET); err != nil {
		return fmt.Errorf("failed to enter container netns: %v", err)
	}
	defer util.SetNS(parentNS, syscall.CLONE_NEWNET)

	link, err := netlink.LinkByName(ifName)
	if err != nil {
		return fmt.Errorf("failed to lookup %q: %v", ifName, err)
	}

	addr := &netlink.Addr{ipn, ""}
	if err = netlink.AddrAdd(link, addr); err != nil {
		return fmt.Errorf("failed to add IP addr to %q: %v", ifName, err)
	}

	return nil
}

func assignIP(parentNS, contNS *os.File, n *Net, ifName string) error {
	if n.IPAlloc.Type == "static" {
		_, ipn, err := net.ParseCIDR(n.IPAlloc.Subnet)
		if err != nil {
			// TODO: cleanup
			return fmt.Errorf("error parsing %q conf: ipAlloc.Subnet: %v")
		}

		ipn.IP, err = allocIP(ipn)
		if err != nil {
			// TODO: cleanup
			return fmt.Errorf("error allocating IP in %v: %v", ipn, err)
		}

		if err = setContIfAddr(parentNS, contNS, ifName, ipn); err != nil {
			return err
		}
	}

	return nil
}

func execPlugins(parentNS, contNS *os.File, contID types.UUID, netns string) (bool, error) {
	fmt.Println("Executing net plugins")

	plugins, err := LoadNetPlugins()
	if err != nil {
		return false, fmt.Errorf("error loading plugin definitions: %v", err)
	}

	nets, err := LoadNets()
	if err != nil {
		return false, fmt.Errorf("error loading network definitions: %v", err)
	}

	hasNet := false

	for i, n := range nets {
		plugin, ok := plugins[n.Type]
		if !ok {
			fmt.Fprintf(os.Stderr, "warning: could not find network plugin %q\n", n.Type)
			continue
		}

		ifName := fmt.Sprintf(ifnamePattern, i)

		fmt.Println("Executing net-plugin ", n.Type)
		if err := plugin.Add(n, contID.String(), netns, n.Filename, "", ifName); err != nil {
			fmt.Fprintf(os.Stderr, "error adding network %q: %v\n", n.Name, err)
			continue
		}

		if err := assignIP(parentNS, contNS, n, ifName); err != nil {
			fmt.Fprintf(os.Stderr, "%s", err)
			continue
		}

		hasNet = true
	}

	fmt.Println("Done executing net plugins")

	return hasNet, nil
}

func bindMountNetns(src string, contID types.UUID) (string, error) {
	contNSPath := fmt.Sprintf("/var/lib/rkt/containers/%v/ns", contID)
	if err := os.Mkdir(contNSPath, 0755); err != nil {
		return "", err
	}

	contNSPath += "/net"

	// mount point has to be an existing file
	f, err := os.Create(contNSPath)
	if err != nil {
		return "", err
	}
	f.Close()

	if err := syscall.Mount(src, contNSPath, "none", syscall.MS_BIND, ""); err != nil {
		return "", err
	}

	return contNSPath, nil
}

func Setup(contID types.UUID) (net.IP, *os.File, error) {
	parentNS, contNS, err := newNetNS()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create new netns: %v", err)
	}
	defer parentNS.Close()
	// we're in childNS!!

	contNSPath, err := bindMountNetns(contNS.Name(), contID)
	if err != nil {
		return nil, nil, err
	}

	if err := util.SetNS(parentNS, syscall.CLONE_NEWNET); err != nil {
		return nil, nil, err
	}

	hasNet, err := execPlugins(parentNS, contNS, contID, contNSPath)
	if err != nil {
		return nil, nil, err
	}

	if err := util.SetNS(contNS, syscall.CLONE_NEWNET); err != nil {
		return nil, nil, err
	}

	if err := loUp(); err != nil {
		return nil, nil, err
	}

	ip, err := allocIP(DefaultIPNet)
	if err != nil {
		return nil, nil, err
	}

	ipn := &net.IPNet{
		IP:   ip,
		Mask: net.CIDRMask(32, 32),
	}

	if !hasNet {
		fmt.Println("No networks detected, setting up default")

		_, contVeth, err := util.SetupVeth(contID.String(), "eth0", ipn, parentNS)
		if err != nil {
			return nil, nil, err
		}

		if err = addDefaultRoute(ip, contVeth); err != nil {
			return nil, nil, fmt.Errorf("failed to add default route: %v", err)
		}

		if err = util.SetNS(parentNS, syscall.CLONE_NEWNET); err != nil {
			return nil, nil, fmt.Errorf("failed to switch to root netns: %v", err)
		}
	} else {
		hostVeth, contVeth, err := util.SetupVeth(contID.String()+"md", "md0", ipn, parentNS)
		if err != nil {
			return nil, nil, err
		}

		err = netlink.RouteAdd(&netlink.Route{
			LinkIndex: contVeth.Attrs().Index,
			Scope:     netlink.SCOPE_UNIVERSE,
			Dst:       ipn,
		})

		if err != nil {
			err = fmt.Errorf("failed to add IP addr to veth: %v", err)
			return nil, nil, err
		}

		if err = util.SetNS(parentNS, syscall.CLONE_NEWNET); err != nil {
			return nil, nil, fmt.Errorf("failed to switch to root netns: %v", err)
		}

		hostVeth, err = netlink.LinkByName(hostVeth.Attrs().Name)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to lookup %q: %v", hostVeth.Attrs().Name, err)
		}

		err = netlink.RouteAdd(&netlink.Route{
			LinkIndex: hostVeth.Attrs().Index,
			Scope:     netlink.SCOPE_UNIVERSE,
			Dst:       ipn,
		})

		if err != nil {
			return nil, nil, fmt.Errorf("failed to add route on host: %v", err)
		}
	}

	//return ip.String(), nil
	return ip, contNS, nil
}

func Teardown(contID types.UUID, ip net.IP) error {
	// TODO

	return nil
}

func Enter(netns *os.File) error {
	return util.SetNS(netns, syscall.CLONE_NEWNET)
}
