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

	baseconfig "github.com/coreos/rkt/pkg/config"
	"github.com/hashicorp/errwrap"
)

const (
	discardNetPluginsKey = "discard"
)

type pathsV1NetPluginDirs struct {
	DiscardPrevious *bool    `json:"discardPrevious"`
	Paths           []string `json:"paths"`
}

type pathsV1 struct {
	NetPluginDirs pathsV1NetPluginDirs `json:"netPluginDirs"`
}

func (p *pathsV1JSONParser) Parse(pi *baseconfig.PathIndex, raw []byte) error {
	paths := pathsV1{}
	if err := json.Unmarshal(raw, &paths); err != nil {
		return errwrap.Wrap(fmt.Errorf("error loading %s", pi.FilePath()), err)
	}
	dirs := &paths.NetPluginDirs
	if err := p.handleDiscard(pi, dirs); err != nil {
		return err
	}
	if err := p.handlePaths(pi, dirs); err != nil {
		return err
	}
	return nil
}

func (p *pathsV1JSONParser) propagateConfig(config *Config) {
	for i, subconfig := range p.configs {
		discard := false
		if value, isSet := p.getPropOpts(i)[discardNetPluginsKey]; isSet {
			discard = value.(bool)
		}
		if discard {
			config.PluginDirs = subconfig.PluginDirs
		} else {
			config.PluginDirs = append(subconfig.PluginDirs, config.PluginDirs...)
		}
	}
}

func (p *pathsV1JSONParser) handleDiscard(pi *baseconfig.PathIndex, dirs *pathsV1NetPluginDirs) error {
	if dirs.DiscardPrevious == nil {
		return nil
	}
	opts := p.getPropOpts(pi.Index)
	if _, isSet := opts[discardNetPluginsKey]; isSet {
		return fmt.Errorf("discarding of previous network plugin directories were already set in the config directory %q", pi.Path)
	}
	opts[discardNetPluginsKey] = *dirs.DiscardPrevious
	return nil
}

func (p *pathsV1JSONParser) handlePaths(pi *baseconfig.PathIndex, dirs *pathsV1NetPluginDirs) error {
	if dirs.Paths == nil {
		return nil
	}
	cfg := p.getConfig(pi.Index)
	if cfg.PluginDirs != nil {
		return fmt.Errorf("network plugin dirs were already set in the config directory %q", pi.Path)
	}
	for i, p := range dirs.Paths {
		if !filepath.IsAbs(p) {
			return fmt.Errorf("network plugin path nr %d (%s) is not absolute", i+1, p)
		}
	}
	cfg.PluginDirs = dirs.Paths
	return nil
}
