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
	"path/filepath"

	nettypes "github.com/coreos/rkt/networking/types"
	baseconfig "github.com/coreos/rkt/pkg/config"
)

type Networks struct {
	ByName  map[string]*nettypes.ActiveNet
	Ordered []*nettypes.ActiveNet
}

// Config is a single place where configuration for stage1 networking
// resides.
type Config struct {
	Networks Networks
}

func newConfig() *Config {
	return &Config{
		Networks: Networks{
			ByName: make(map[string]*nettypes.ActiveNet),
		},
	}
}

type activeNetsSortableByPath []*nettypes.ActiveNet

func (an activeNetsSortableByPath) Len() int {
	return len(an)
}

func (an activeNetsSortableByPath) Less(i, j int) bool {
	return filepath.Base(an[i].Runtime.ConfPath) < filepath.Base(an[j].Runtime.ConfPath)
}

func (an activeNetsSortableByPath) Swap(i, j int) {
	an[i], an[j] = an[j], an[i]
}

type parserBase struct {
	configs []*Config
}

func (p *parserBase) getConfig(idx int) *Config {
	for len(p.configs) <= idx {
		p.configs = append(p.configs, newConfig())
	}
	return p.configs[idx]
}

func (p *parserBase) visited() bool {
	return len(p.configs) > 0
}

type configPropagator interface {
	propagateConfig(config *Config)
	visited() bool
}

// the parsers below implement both baseconfig.Parser and
// configPropagator interfaces

type oldConfJSONParser struct {
	parserBase
}

var _ baseconfig.Parser = (*oldConfJSONParser)(nil)
var _ configPropagator = (*oldConfJSONParser)(nil)

// the config type for conf files, implements
// github.com/coreos/rkt/pkg/config Type

type oldConfJSON struct{}

var _ baseconfig.Type = (*oldConfJSON)(nil)

func (*oldConfJSON) Extension() string {
	return "conf"
}

func (*oldConfJSON) GetKindAndVersion(raw []byte) (string, string, error) {
	return "old-conf", "old-conf", nil
}
