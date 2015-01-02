package network

import (
	"crypto/rand"
	"encoding/binary"
	"math/big"
	"net"
)

var DefaultIPNet *net.IPNet

func init() {
	_, DefaultIPNet, _ = net.ParseCIDR("172.16.28.0/24")
}

func ipAdd(ip net.IP, val uint) net.IP {
	n := binary.BigEndian.Uint32(ip.To4())
	n += uint32(val)

	nip := make([]byte, 4)
	binary.BigEndian.PutUint32(nip, n)
	return net.IP(nip)
}

func allocIP(ipn *net.IPNet) (net.IP, error) {
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

func deallocIP(ip net.IP) error {
	return nil
}
