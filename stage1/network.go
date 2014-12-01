package main

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/coreos/go-systemd/unit"
)

func setupNetwork(c *Container) (string, error) {
	_, brIPNet, err := net.ParseCIDR(c.BridgeAddr)
	if err != nil {
		return "", err
	}

	// if brIPNet=10.1.0.0/16, bridge will get 10.1.0.1
	brIPNet.IP = ipAdd(brIPNet.IP, 1)

	// setup the rkt0 bridge
	// ignore error b/c rkt0 might exist
	// TODO(eyakubovich): move to use netlink lib
	err = exec.Command("brctl", "addbr", c.Bridge).Run()
	if err == nil {
		// setup br0 IP
		err := exec.Command("ip", "addr", "add", brIPNet.String(), "dev", c.Bridge).Run()
		if err != nil {
			return "", fmt.Errorf("Failed to add IP addr to bridge: %v", err)
		}

		// bring bridge up
		err = exec.Command("ip", "link", "set", c.Bridge, "up").Run()
		if err != nil {
			return "", fmt.Errorf("Failed to set bridge to UP state: %v", err)
		}
	} else {
		fmt.Println("warning: brctl failed: ", err)
	}

	// allocate IP
	ip, err := pickIP(brIPNet)
	if err != nil {
		return "", fmt.Errorf("Failed to allocate IP: %v", err)
	}

	fmt.Println("Allocated IP: ", ip)

	// write out network.service file
	err = writeNetworkFile(c, ip, brIPNet)
	if err != nil {
		return "", fmt.Errorf("Failed to write network.service file: %v", err)
	}

	brPort := "vb-" + c.MachineName // nspawn names host end "vb-<machine-name>"

	// setup ebtables to prevent IP-spoofing
	err = antiSpoof("-I", brPort, ip.String())
	if err != nil {
		return "", fmt.Errorf("Failed to setup ebtables: %v", err)
	}

	return ip.String(), nil
}

func writeNetworkFile(c *Container, ip net.IP, gw *net.IPNet) error {
	size, _ := gw.Mask.Size()
	cmd1 := fmt.Sprintf("/bin/ip addr add %v/%v dev host0", ip, size)
	cmd2 := "/bin/ip link set host0 up"
	cmd3 := fmt.Sprintf("/bin/ip route add default via %s dev host0", gw.IP.String())

	opts := []*unit.UnitOption{
		&unit.UnitOption{"Unit", "Description", "Setup networking"},
		&unit.UnitOption{"Unit", "DefaultDependencies", "false"},
		&unit.UnitOption{"Service", "Type", "oneshot"},
		&unit.UnitOption{"Service", "ExecStart", cmd1},
		&unit.UnitOption{"Service", "ExecStart", cmd2},
		&unit.UnitOption{"Service", "ExecStart", cmd3},
	}

	file, err := os.OpenFile(filepath.Join(ServicesPath(c.Root), "network.service"), os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("failed to create service file: %v", err)
	}
	defer file.Close()

	_, err = io.Copy(file, unit.Serialize(opts))
	if err != nil {
		return fmt.Errorf("failed to write service file: %v", err)
	}

	src := filepath.Join("..", "network.service")
	dst := WantUnitLinkPath(c.Root, "network.service")
	if err := os.Symlink(src, dst); err != nil {
		return fmt.Errorf("failed to link service want: %v", err)
	}

	return nil
}

func ipAdd(ip net.IP, val uint) net.IP {
	n := binary.BigEndian.Uint32(ip.To4())
	n += uint32(val)

	nip := make([]byte, 4)
	binary.BigEndian.PutUint32(nip, n)
	return net.IP(nip)
}

func pickIP(ipn *net.IPNet) (net.IP, error) {
	ones, bits := ipn.Mask.Size()
	zeros := bits - ones
	rng := (1 << uint(zeros)) - 2 // (reduce for gw, bcast)

	n, err := rand.Int(rand.Reader, big.NewInt(int64(rng)))
	if err != nil {
		return nil, err
	}

	offset := uint(n.Uint64() + 1)

	return ipAdd(ipn.IP, offset), nil
}

func antiSpoof(cmd, brPort, ipAddr string) error {
	// ebtables is not available in stage1 rootfs for now.
	// commented out until added

	//args := []string{"-t", "filter", cmd, "INPUT", "-i", brPort, "-p", "IPV4", "!", "--ip-source", ipAddr, "-j", "DROP"}
	//return exec.Command("ebtables", args...).Run()
	return nil
}

func teardownNetwork(c *Container, ipAddr string) error {
	brPort := "vb-" + c.MachineName // nspawn names host end "vb-<machine-name>"
	err := antiSpoof("-D", brPort, ipAddr)
	if err != nil {
		return fmt.Errorf("Failed to setup ebtables: %v", err)
	}

	return nil
}
