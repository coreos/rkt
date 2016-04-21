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
	"errors"

	"github.com/coreos/rkt/common"
	baseconfig "github.com/coreos/rkt/pkg/config"
)

type configSetup struct {
	oldDirectory   *baseconfig.Directory
	oldPropagators []configPropagator

	newDirectory   *baseconfig.Directory
	newPropagators []configPropagator

	// propagators will be set to either newPropagators or
	// oldPropagators when we will know which of newDirectory or
	// oldDirectory was used
	propagators []configPropagator
}

func GetConfigFrom(dirs ...string) (*Config, error) {
	setup, err := getConfigSetup()
	if err != nil {
		return nil, err
	}
	var filteredDirs []string
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		filteredDirs = append(filteredDirs, dir)
	}
	if len(filteredDirs) == 0 {
		return nil, errors.New("no valid directories to get the configuration from")
	}
	return setup.run(filteredDirs...)
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
	newDir, newProps, err := getNewSetup()
	if err != nil {
		return nil, err
	}
	setup := &configSetup{
		oldDirectory:   oldDir,
		oldPropagators: oldProps,

		newDirectory:   newDir,
		newPropagators: newProps,
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
	dir := baseconfig.NewDirectory(common.DeprecatedCDB, &oldConfJSON{})
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

func getNewSetup() (*baseconfig.Directory, []configPropagator, error) {
	networkV1 := &networkV1JSONParser{}
	pathsV1 := &pathsV1JSONParser{}
	dir := common.NewConfigDirectory(common.Stage1CDB)
	parsers := []*baseconfig.ParserSetup{
		{
			Kind:    "network",
			Version: "v1",
			Parser:  networkV1,
		},
		{
			Kind:    "paths",
			Version: "v1",
			Parser:  pathsV1,
		},
	}
	subdirs := []*baseconfig.SubdirSetup{
		{
			Subdir: "net.d",
			Kinds:  []string{"network"},
		},
		{
			Subdir: "paths.d",
			Kinds:  []string{"paths"},
		},
	}
	if err := dir.RegisterParsers(parsers); err != nil {
		return nil, nil, err
	}
	if err := dir.RegisterSubdirectories(subdirs); err != nil {
		return nil, nil, err
	}
	props := []configPropagator{
		networkV1,
		pathsV1,
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
	if s.newDirectory != nil {
		if err := s.newDirectory.WalkDirectories(dirs...); err != nil {
			return err
		}
		for _, p := range s.newPropagators {
			if p.visited() {
				s.propagators = s.newPropagators
				return nil
			}
		}
	}
	s.propagators = s.oldPropagators
	return s.oldDirectory.WalkDirectories(dirs...)
}

func (s *configSetup) propagateChanges(cfg *Config) {
	for _, p := range s.propagators {
		p.propagateConfig(cfg)
	}
}
