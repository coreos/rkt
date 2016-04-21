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

package config

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/coreos/rkt/networking/netinfo"
	nettypes "github.com/coreos/rkt/networking/types"
	baseconfig "github.com/coreos/rkt/pkg/config"
	"github.com/hashicorp/errwrap"
)

type networkV1 struct {
	Priority *int                   `json:"priority"`
	Name     string                 `json:"name"`
	CNIConf  map[string]interface{} `json:"cniConf"`
	//CNIConf  *nettypes.NetConf `json:"cniConf"`
}

func (p *networkV1JSONParser) Parse(pi *baseconfig.PathIndex, raw []byte) error {
	network := networkV1{}
	fp := pi.FilePath()
	if err := json.Unmarshal(raw, &network); err != nil {
		return errwrap.Wrap(fmt.Errorf("error loading %s", fp), err)
	}
	if err := p.validateNetworkV1(&network, fp); err != nil {
		return err
	}
	cfg := p.getConfig(pi.Index)
	name := network.Name
	if _, ok := cfg.Networks.ByName[name]; ok {
		return fmt.Errorf("network %q was already defined in %q", name, pi.Path)
	}
	an, err := p.getActiveNet(&network, fp)
	if err != nil {
		return err
	}
	cfg.Networks.Ordered = append(cfg.Networks.Ordered, an)
	cfg.Networks.ByName[name] = an
	return nil
}

func (p *networkV1JSONParser) propagateConfig(config *Config) {
	networks := &config.Networks
	for _, subconfig := range p.configs {
		subnetworks := &subconfig.Networks
		for name, subnetwork := range subnetworks.ByName {
			if network, ok := networks.ByName[name]; ok {
				if subnetwork.Conf != nil {
					*network = *subnetwork
				} else {
					// only a priority has
					// changed, it affects how the
					// file will be named inside
					// the pod
					network.Runtime = subnetwork.Runtime
				}
			} else {
				networks.Ordered = append(networks.Ordered, subnetwork)
				networks.ByName[name] = subnetwork
			}
		}
	}
	// since "cniConf" field is optional, it is possible to get an
	// entry with no configuration at all, remove those
	var indices []int
	ordered := networks.Ordered
	for i, network := range ordered {
		if network.Conf == nil {
			indices = append([]int{i}, indices...)
		}
	}
	for _, i := range indices {
		name := ordered[i].Runtime.NetName
		delete(networks.ByName, name)
		ordered[i] = ordered[len(ordered)-1]
		ordered[len(ordered)-1] = nil
		ordered = ordered[:len(ordered)-1]
	}
	// We need to sort it, because baseconfig does not guarantee
	// any ordering of visited files in the directory, but such
	// ordering was assumed in the old code.
	sort.Sort(activeNetsSortableByPath(ordered))
	networks.Ordered = ordered
}

func (p *networkV1JSONParser) validateNetworkV1(network *networkV1, path string) error {
	if network.Priority == nil {
		return fmt.Errorf("network config file at %q is missing a priority field", path)
	}
	prio := *network.Priority
	if prio < 0 || prio > 99 {
		return fmt.Errorf("a priority field in network config file at %q is invalid (%d)", path, prio)
	}
	if network.Name == "" {
		return fmt.Errorf("network config file at %q is missing a name field or it is empty", path)
	}
	if cniName, hasName := network.CNIConf["name"]; hasName {
		if cniName != network.Name {
			return fmt.Errorf("conflicting network names in %q (%s) and %q (%s) fields in %q (no need to specify the latter, likely a leftover from a conversion)", "name", network.Name, "cniConf.name", cniName, path)
		}
		// TODO: stderr.Printf("warning: no need to specify a name in cniConf section, likely a leftover from a conversion")
	}
	return nil
}

func (p *networkV1JSONParser) getActiveNet(network *networkV1, path string) (*nettypes.ActiveNet, error) {
	conf, raw, err := p.getFixedConf(network)
	if err != nil {
		return nil, err
	}
	baseNoExt := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	oldStylePath := fmt.Sprintf("%02d-%s.conf", *network.Priority, baseNoExt)
	an := &nettypes.ActiveNet{
		ConfBytes: raw,
		Conf:      conf,
		Runtime: &netinfo.NetInfo{
			NetName:  network.Name,
			ConfPath: oldStylePath,
		},
	}
	return an, nil
}

func (p *networkV1JSONParser) getFixedConf(network *networkV1) (*nettypes.NetConf, []byte, error) {
	conf := network.CNIConf
	if conf == nil {
		return nil, nil, nil
	}
	conf["name"] = network.Name
	raw, err := json.Marshal(conf)
	if err != nil {
		return nil, nil, err
	}
	nc := &nettypes.NetConf{}
	if err := json.Unmarshal(raw, nc); err != nil {
		return nil, nil, err
	}
	return nc, raw, nil
}
