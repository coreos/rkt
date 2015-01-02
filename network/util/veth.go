package util

import (
	"crypto/sha512"
	"fmt"
	"net"
	"os"

	"github.com/coreos/rocket/Godeps/_workspace/src/github.com/vishvananda/netlink"
)

func makeVeth(name, peer string) (netlink.Link, error) {
	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name:  name,
			Flags: net.FlagUp,
		},
		PeerName: peer,
	}
	if err := netlink.LinkAdd(veth); err != nil {
		return nil, err
	}

	return veth, nil
}

func hash(s string) string {
	h := sha512.New()
	h.Write([]byte(s))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// Should be in container netns
func SetupVeth(contID, contVethName string, ipn *net.IPNet, parentNS *os.File) (hostVeth, contVeth netlink.Link, err error) {
	hostVethName := "rkt" + hash(contID)[:5]
	hostVeth, err = makeVeth(hostVethName, contVethName)
	if err != nil {
		err = fmt.Errorf("failed to make veth pair: %v", err)
		return
	}

	if err = netlink.LinkSetNsFd(hostVeth, int(parentNS.Fd())); err != nil {
		err = fmt.Errorf("failed to move veth to root netns: %v", err)
		return
	}

	contVeth, err = netlink.LinkByName(contVethName)
	if err != nil {
		err = fmt.Errorf("failed to lookup %q: %v", contVethName, err)
		return
	}

	if err = netlink.LinkSetUp(contVeth); err != nil {
		err = fmt.Errorf("failed to set eth0 up: %v", err)
		return
	}

	if ipn != nil {
		addr := &netlink.Addr{ipn, ""}
		if err = netlink.AddrAdd(contVeth, addr); err != nil {
			err = fmt.Errorf("failed to add IP addr to veth: %v", err)
			return
		}
	}

	return
}
