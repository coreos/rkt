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
	"fmt"
	"net"
	"strconv"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/coreos/go-iptables/iptables"
)

func (e *podEnv) forwardPorts(fps []ForwardedPort, defIP net.IP) error {
	if len(fps) == 0 {
		return nil
	}

	ipt, err := iptables.New()
	if err != nil {
		return err
	}

	// Create a separate chain for this pod. This helps with debugging
	// and makes it easier to cleanup
	chain := e.portFwdChain()

	if err = ipt.NewChain("nat", chain); err != nil {
		return err
	}

	rule := e.portFwdRuleSpec(chain)

	// outside traffic hitting this host
	if err = ipt.AppendUnique("nat", "PREROUTING", rule...); err != nil {
		return err
	}

	// traffic originating on this host
	if err = ipt.AppendUnique("nat", "OUTPUT", rule...); err != nil {
		return err
	}

	for _, p := range fps {
		if err = forwardPort(ipt, chain, &p, defIP); err != nil {
			return err
		}
	}

	return nil
}

func forwardPort(ipt *iptables.IPTables, chain string, p *ForwardedPort, defIP net.IP) error {
	dst := fmt.Sprintf("%v:%v", defIP, p.PodPort)
	dport := strconv.Itoa(int(p.HostPort))

	return ipt.AppendUnique("nat", chain, "-p", p.Protocol, "--dport", dport, "-j", "DNAT", "--to-destination", dst)
}

func (e *podEnv) unforwardPorts() error {
	ipt, err := iptables.New()
	if err != nil {
		return err
	}

	chain := e.portFwdChain()

	rule := e.portFwdRuleSpec(chain)

	// There's no clean way now to test if a chain exists or
	// even if a rule exists if the chain is not present.
	// So we swallow the errors for now :(
	// TODO(eyakubovich): move to using libiptc for iptable
	// manipulation

	// outside traffic hitting this hot
	ipt.Delete("nat", "PREROUTING", rule...)

	// traffic originating on this host
	ipt.Delete("nat", "OUTPUT", rule...)

	// there should be no references, delete the chain
	ipt.ClearChain("nat", chain)
	ipt.DeleteChain("nat", chain)

	return nil
}

func (e *podEnv) portFwdChain() string {
	return "RKT-PFWD-" + e.podID.String()[0:8]
}

func (e *podEnv) portFwdRuleSpec(chain string) []string {
	return []string{"-m", "addrtype", "--dst-type", "LOCAL", "-j", chain}
}
