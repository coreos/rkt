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

package netinfo

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"syscall"

	cniv031 "github.com/containernetworking/cni/pkg/types/current"
)

const filename = "net-info.json"

// A type and some structure to represent rkt's view of a *runtime*
// network instance.
//
// Each instance represents a network configuration that has been enabled,
// along with runtime information from the network plugin.
//
// This information is also serialized in the pod's runtime directory so that
// `rkt list` and other stage0 programs can access the runtime state.
type NetInfo struct {
	NetName string `json:"netName"`
	// We copy in the configuration file to the pod root - this is that path
	ConfPath string      `json:"netConf"`
	IfName   string      `json:"ifName"`
	IPs      []net.IPNet `json:"ips"`
	Args     [][2]string `json:"args"`

	// This is so we can parse older netinfo files
	legacyIP   net.IP `json:"ip,omitempty"`
	legacyMask net.IP `json:"mask,omitempty"`

	// Don't need to serialize this - only used during init
	CniResult *cniv031.Result `json:"-"`
}

func LoadAt(cdirfd int) ([]NetInfo, error) {
	fd, err := syscall.Openat(cdirfd, filename, syscall.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}

	f := os.NewFile(uintptr(fd), filename)

	var info []NetInfo
	if err := json.NewDecoder(f).Decode(&info); err != nil {
		return info, err
	}

	for _, ni := range info {
		ni.fixupLegacy()
	}

	return info, err
}

// fixupIPs up-converts older version netinfo structs
func (ni *NetInfo) fixupLegacy() {
	if len(ni.IPs) == 0 && ni.legacyIP != nil {
		ni.IPs = []net.IPNet{
			{
				IP:   ni.legacyIP,
				Mask: net.IPMask(ni.legacyMask),
			},
		}
	}
}

func Save(root string, info []NetInfo) error {
	f, err := os.Create(filepath.Join(root, filename))
	if err != nil {
		return err
	}
	defer f.Close()

	return json.NewEncoder(f).Encode(info)
}

// MergeCNIResult will incorporate the result of a CNI plugin's execution
func (ni *NetInfo) MergeCNIResult(result *cniv031.Result) {
	ni.CniResult = result

	for _, ip := range result.IPs {
		// Skip non-container IPs
		if ip.Interface >= 0 && ip.Interface < len(result.Interfaces) && result.Interfaces[ip.Interface].Sandbox == "" {
			continue
		}
		ni.IPs = append(ni.IPs, ip.Address)
	}
}

func (ni *NetInfo) FirstIP() net.IP {
	for _, n := range ni.IPs {
		return n.IP
	}
	return nil
}

func (ni *NetInfo) FirstIP4() net.IP {
	for _, n := range ni.IPs {
		ip := n.IP.To4()
		if ip != nil {
			return ip
		}
	}
	return nil
}

func (ni *NetInfo) FirstIPConfig() *cniv031.IPConfig {
	for _, ip := range ni.CniResult.IPs {
		if ip.Interface >= 0 && ip.Interface < len(ni.CniResult.Interfaces) && ni.CniResult.Interfaces[ip.Interface].Sandbox == "" {
			continue
		}
		return ip
	}
	return nil
}
