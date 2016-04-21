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
	"github.com/coreos/rkt/common"
	baseconfig "github.com/coreos/rkt/pkg/config"
)

type configSetup struct {
	oldDirectory   *baseconfig.Directory
	oldPropagators []configPropagator
}

func GetConfigFrom(dirs ...string) (*Config, error) {
	setup, err := getConfigSetup()
	if err != nil {
		return nil, err
	}
	return setup.run(dirs...)
}

func GetPodConfig(podRoot string) (*Config, error) {
	setup, err := getPodConfigSetup()
	if err != nil {
		return nil, err
	}
	return setup.run(podRoot)
}

func getConfigSetup() (*configSetup, error) {
	oldDir, oldProps, err := getOldSetup()
	if err != nil {
		return nil, err
	}
	setup := &configSetup{
		oldDirectory:   oldDir,
		oldPropagators: oldProps,
	}
	return setup, nil
}

func getPodConfigSetup() (*configSetup, error) {
	dir, props, err := getOldSetup()
	if err != nil {
		return nil, err
	}
	setup := &configSetup{
		oldDirectory:   dir,
		oldPropagators: props,
	}
	return setup, nil
}

// this function should be considered frozen
func getOldSetup() (*baseconfig.Directory, []configPropagator, error) {
	dir := baseconfig.NewDirectory(common.CDB, &oldConfJSON{})
	parser := &oldConfJSONParser{}
	if err := dir.RegisterParser("old-conf", "old-conf", parser); err != nil {
		return nil, nil, err
	}
	if err := dir.RegisterSubdirectory("net.d", []string{"old-conf"}); err != nil {
		return nil, nil, err
	}
	props := []configPropagator{
		parser,
	}
	return dir, props, nil
}

func (s *configSetup) run(dirs ...string) (*Config, error) {
	if err := s.walkDirectories(dirs...); err != nil {
		return nil, err
	}
	cfg := newConfig()
	s.propagateChanges(cfg)
	return cfg, nil
}

func (s *configSetup) walkDirectories(dirs ...string) error {
	return s.oldDirectory.WalkDirectories(dirs...)
}

func (s *configSetup) propagateChanges(cfg *Config) {
	for _, p := range s.oldPropagators {
		p.propagateConfig(cfg)
	}
}
