// Copyright 2016 The rkt Authors
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
	"net"
	"strconv"
	"strings"

	"github.com/appc/spec/schema/types"
	"github.com/hashicorp/errwrap"
)

// Handle parsing some CLI flags:
//
// --port
// --raw-port
// along with the flagStringList, used everywhere

// flagStringList implements the flag.Value interface to contain a set of strings
type flagStringList []string

func (dns *flagStringList) Set(s string) error {
	*dns = append(*dns, s)
	return nil
}

func (dns *flagStringList) String() string {
	return strings.Join(*dns, " ")
}

func (dns *flagStringList) Type() string {
	return "flagStringList"
}

// portList parses the --port flag to map host ports to named ports in the manifest.
// the format is --port=<NAME>:<HOSTPORT>, e.g. --port=http:8080, or <NAME>:<HOSTIP>:<HOSTPORT>
type portList []types.ExposedPort

func (pl *portList) Set(s string) error {
	parts := strings.SplitN(s, ":", 3)
	if len(parts) < 2 {
		return fmt.Errorf("%q is not in name:[ip:]port format", s)
	}

	name, err := types.NewACName(parts[0])
	if err != nil {
		return errwrap.Wrap(fmt.Errorf("%q is not a valid port name", parts[0]), err)
	}

	portStr := parts[1]
	var ip net.IP

	// If an IP was supplied, parse it
	if len(parts) == 3 {
		portStr = parts[2]
		ip = net.ParseIP(parts[1])
		if ip == nil {
			return fmt.Errorf("%q is not a valid IP", parts[1])
		}
	}

	port, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return fmt.Errorf("%q is not a valid port number", parts[1])
	}

	p := types.ExposedPort{
		Name:     *name,
		HostPort: uint(port),
		HostIP:   ip,
	}

	*pl = append(*pl, p)
	return nil
}

func (pl *portList) String() string {
	var ps []string
	for _, p := range []types.ExposedPort(*pl) {
		ps = append(ps, fmt.Sprintf("%v:%v", p.Name, p.HostPort))
	}
	return strings.Join(ps, " ")
}

func (pl *portList) Type() string {
	return "portList"
}

func (pl *portList) Help() string {
	return "ports to expose on the host (requires contained network). Syntax: --port=NAME:HOSTPORT or --port=NAME:HOSTIP:HOSTPORT"
}

// rawPortList is a port specified that does not reference the manifest.
// The format is name:proto:podPort:hostIP:hostPort
// e.g. http:tcp:8080:0.0.0.0:80
type rawPortList []types.ExposedPort

func (pl *rawPortList) Set(s string) error {
	parts := strings.SplitN(s, ":", 5)
	if len(parts) != 5 {
		return fmt.Errorf("--port invalid format")
	}

	// parsey parsey
	name, err := types.NewACName(parts[0])
	if err != nil {
		return err
	}

	proto := parts[1]
	switch proto {
	case "tcp", "udp":
	default:
		return fmt.Errorf("invalid protocol %q", proto)
	}

	p, err := strconv.ParseUint(parts[2], 10, 16)
	if err != nil {
		return err
	}
	podPortNo := uint(p)

	ip := net.ParseIP(parts[3])
	if ip == nil {
		return fmt.Errorf("could not parse IP %q", ip)
	}

	p, err = strconv.ParseUint(parts[4], 10, 16)
	if err != nil {
		return err
	}
	hostPortNo := uint(p)

	podSide := types.Port{
		Name:            *name,
		Protocol:        proto,
		Port:            podPortNo,
		Count:           1,
		SocketActivated: false,
	}

	hostSide := types.ExposedPort{
		Name:     *name,
		HostPort: hostPortNo,
		HostIP:   ip,
		PodPort:  &podSide,
	}

	*pl = append(*pl, hostSide)
	return nil
}

func (pl *rawPortList) String() string {
	ss := make([]string, 0, len(*pl))
	for _, p := range *pl {
		ss = append(ss, fmt.Sprintf("%s:%s:%d:%s:%d",
			p.Name, p.PodPort.Protocol, p.PodPort.Port,
			p.HostIP, p.HostPort))

	}
	return strings.Join(ss, ",")
}

func (pl *rawPortList) Type() string {
	return "rawPortList"
}

func (pl *rawPortList) Help() string {
	return `raw ports to forward (ignores manifest). format: "name:proto:podPort:hostIP:hostPort"`
}
