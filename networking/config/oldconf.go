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

func (p *oldConfJSONParser) Parse(pi *baseconfig.PathIndex, raw []byte) error {
	n := &nettypes.NetConf{}
	fp := pi.FilePath()
	if err := json.Unmarshal(raw, n); err != nil {
		return errwrap.Wrap(fmt.Errorf("error loading %s", fp), err)
	}
	cfg := p.getConfig(pi.Index)
	if an, ok := cfg.Networks.ByName[n.Name]; ok {
		if an.Runtime.ConfPath < fp {
			// TODO: stderr.Printf("%q network already defined, ignoring %v", n.Name, fp)
			return nil
		}
		*an = *p.getActiveNet(raw, n, fp)
	} else {
		an := p.getActiveNet(raw, n, fp)
		cfg.Networks.Ordered = append(cfg.Networks.Ordered, an)
		cfg.Networks.ByName[n.Name] = an
	}
	return nil
}

func (p *oldConfJSONParser) propagateConfig(config *Config) {
	for _, subconfig := range p.configs {
		for name, an := range subconfig.Networks.ByName {
			if an2, exists := config.Networks.ByName[name]; exists {
				*an2 = *an
			} else {
				config.Networks.ByName[name] = an
				config.Networks.Ordered = append(config.Networks.Ordered, an)
			}
		}
	}
	// We need to sort it, because baseconfig does not guarantee
	// any ordering of visited files in the directory, but such
	// ordering was assumed in the old code.
	sort.Sort(activeNetsSortableByPath(config.Networks.Ordered))
	// We need to fixup confpaths, because we can have two
	// configuration files from two different directories that set
	// up networks with different names, but the basenames of the
	// files are the same (e.g. <global>/net.d/10-net.conf and
	// <local>/net.d/10-net.conf). To avoid that, we append a
	// number here (so we will get 10-net-0.conf and
	// 10-net-1.conf).
	fixupConfPaths(config.Networks.Ordered)
}

func (p *oldConfJSONParser) getActiveNet(raw []byte, n *nettypes.NetConf, path string) *nettypes.ActiveNet {
	return &nettypes.ActiveNet{
		ConfBytes: raw,
		Conf:      n,
		Runtime: &netinfo.NetInfo{
			NetName:  n.Name,
			ConfPath: path,
		},
	}
}

func fixupConfPaths(ans []*nettypes.ActiveNet) {
	for idx, an := range ans {
		dir := filepath.Dir(an.Runtime.ConfPath)
		base := filepath.Base(an.Runtime.ConfPath)
		ext := filepath.Ext(base)
		newBase := fmt.Sprintf("%s-%d%s", strings.TrimSuffix(base, ext), idx, ext)
		an.Runtime.ConfPath = filepath.Join(dir, newBase)
	}
}
