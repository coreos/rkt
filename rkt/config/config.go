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

package config

import (
	"encoding/json"
	"errors"
	"net/http"

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

// MarshalJSON marshals the config for user output.
func (c *Config) MarshalJSON() ([]byte, error) {
	stage0 := []interface{}{}

	for host, auth := range c.AuthPerHost {
		var typ string
		var credentials interface{}

		switch h := auth.(type) {
		case *basicAuthHeaderer:
			typ = "basic"
			credentials = h.auth
		case *oAuthBearerTokenHeaderer:
			typ = "oauth"
			credentials = h.auth
		default:
			return nil, errors.New("unknown headerer type")
		}

		auth := struct {
			RktVersion  string      `json:"rktVersion"`
			RktKind     string      `json:"rktKind"`
			Domains     []string    `json:"domains"`
			Type        string      `json:"type"`
			Credentials interface{} `json:"credentials"`
		}{
			RktVersion:  "v1",
			RktKind:     "auth",
			Domains:     []string{host},
			Type:        typ,
			Credentials: credentials,
		}

		stage0 = append(stage0, auth)
	}

	for registry, credentials := range c.DockerCredentialsPerRegistry {
		dockerAuth := struct {
			RktVersion  string           `json:"rktVersion"`
			RktKind     string           `json:"rktKind"`
			Registries  []string         `json:"registries"`
			Credentials BasicCredentials `json:"credentials"`
		}{
			RktVersion:  "v1",
			RktKind:     "dockerAuth",
			Registries:  []string{registry},
			Credentials: credentials,
		}

		stage0 = append(stage0, dockerAuth)
	}

	paths := struct {
		RktVersion   string `json:"rktVersion"`
		RktKind      string `json:"rktKind"`
		Data         string `json:"data"`
		Stage1Images string `json:"stage1-images"`
	}{
		RktVersion:   "v1",
		RktKind:      "paths",
		Data:         c.Paths.DataDir,
		Stage1Images: c.Paths.Stage1ImagesDir,
	}

	stage1 := struct {
		RktVersion string `json:"rktVersion"`
		RktKind    string `json:"rktKind"`
		Name       string `json:"name"`
		Version    string `json:"version"`
		Location   string `json:"location"`
	}{
		RktVersion: "v1",
		RktKind:    "stage1",
		Name:       c.Stage1.Name,
		Version:    c.Stage1.Version,
		Location:   c.Stage1.Location,
	}

	stage0 = append(stage0, paths, stage1)

	data := map[string]interface{}{"stage0": stage0}
	return json.Marshal(data)
}

type configParser interface {
	parse(config *Config, raw []byte) error
}

var (
	// configSubDirs is a map saying what kinds of configuration
	// (values) are acceptable in a config subdirectory (key)
	configSubDirs  = make(map[string][]string)
	parsersForKind = make(map[string]map[string]configParser)
)

// ResolveAuthPerHost takes a map of strings to Headerer and resolves the
// Headerers to http.Headers
func ResolveAuthPerHost(authPerHost map[string]Headerer) map[string]http.Header {
	hostHeaders := make(map[string]http.Header, len(authPerHost))
	for k, v := range authPerHost {
		hostHeaders[k] = v.Header()
	}
	return hostHeaders
}

// GetConfigFrom gets the Config instance with configuration taken
// from given paths. Subsequent paths override settings from the
// previous paths.
func GetConfigFrom(dirs ...string) (*Config, error) {
	setup, err := getConfigSetup()
	if err != nil {
		return nil, err
	}
	if err := setup.walkDirectories(dirs...); err != nil {
		return nil, err
	}
	cfg := newConfig()
	setup.propagateChanges(cfg)
	return cfg, nil
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

type commonSetup struct {
	authV1       *authV1JsonParser
	dockerAuthV1 *dockerAuthV1JsonParser
	pathsV1      *pathsV1JsonParser
	stage1V1     *stage1V1JsonParser

	baseParsers []*baseconfig.ParserSetup
	baseSubdirs []*baseconfig.SubdirSetup
}

// this function should be considered frozen
func getOldSetup() (*baseconfig.Directory, []configPropagator, error) {
	dir := common.NewConfigDirectory(common.DeprecatedCDB)
	setup := getCommonSetup()
	if err := dir.RegisterParsers(setup.baseParsers); err != nil {
		return nil, nil, err
	}
	if err := dir.RegisterSubdirectories(setup.baseSubdirs); err != nil {
		return nil, nil, err
	}
	props := []configPropagator{
		setup.authV1,
		setup.dockerAuthV1,
		setup.pathsV1,
		setup.stage1V1,
	}
	return dir, props, nil
}

func getNewSetup() (*baseconfig.Directory, []configPropagator, error) {
	dir := common.NewConfigDirectory(common.Stage0CDB)
	setup := getCommonSetup()
	additionalParsers := []*baseconfig.ParserSetup{
	// add new parsers here
	}
	additionalSubdirs := []*baseconfig.SubdirSetup{
	// add new subdirs here
	}
	setup.baseParsers = append(setup.baseParsers, additionalParsers...)
	setup.baseSubdirs = append(setup.baseSubdirs, additionalSubdirs...)
	if err := dir.RegisterParsers(setup.baseParsers); err != nil {
		return nil, nil, err
	}
	if err := dir.RegisterSubdirectories(setup.baseSubdirs); err != nil {
		return nil, nil, err
	}
	props := []configPropagator{
		setup.authV1,
		setup.dockerAuthV1,
		setup.pathsV1,
		setup.stage1V1,
		// add new propagators here
	}
	return dir, props, nil
}

// this function should be considered frozen
func getCommonSetup() *commonSetup {
	authV1 := &authV1JsonParser{}
	dockerAuthV1 := &dockerAuthV1JsonParser{}
	pathsV1 := &pathsV1JsonParser{}
	stage1V1 := &stage1V1JsonParser{}
	parsers := []*baseconfig.ParserSetup{
		{
			Kind:    "auth",
			Version: "v1",
			Parser:  authV1,
		},
		{
			Kind:    "dockerAuth",
			Version: "v1",
			Parser:  dockerAuthV1,
		},
		{
			Kind:    "paths",
			Version: "v1",
			Parser:  pathsV1,
		},
		{
			Kind:    "stage1",
			Version: "v1",
			Parser:  stage1V1,
		},
	}
	subdirs := []*baseconfig.SubdirSetup{
		{
			Subdir: "auth.d",
			Kinds:  []string{"auth", "dockerAuth"},
		},
		{
			Subdir: "paths.d",
			Kinds:  []string{"paths"},
		},
		{
			Subdir: "stage1.d",
			Kinds:  []string{"stage1"},
		},
	}
	return &commonSetup{
		authV1:       authV1,
		dockerAuthV1: dockerAuthV1,
		pathsV1:      pathsV1,
		stage1V1:     stage1V1,

		baseParsers: parsers,
		baseSubdirs: subdirs,
	}
}

func (s *configSetup) walkDirectories(dirs ...string) error {
	if err := s.newDirectory.WalkDirectories(dirs...); err != nil {
		return err
	}
	for _, p := range s.newPropagators {
		if p.visited() {
			s.propagators = s.newPropagators
			return nil
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
