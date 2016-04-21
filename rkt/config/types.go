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
	"net/http"

	baseconfig "github.com/coreos/rkt/pkg/config"
)

// Headerer is an interface for getting additional HTTP headers to use
// when downloading data (images, signatures).
type Headerer interface {
	Header() http.Header
}

// BasicCredentials holds typical credentials used for authentication
// (user and password). Used for fetching docker images.
type BasicCredentials struct {
	User     string `json:"user"`
	Password string `json:"password"`
}

// ConfigurablePaths holds various paths defined in the configuration.
type ConfigurablePaths struct {
	DataDir         string
	Stage1ImagesDir string
}

// Stage1 holds name, version and location of a default stage1 image
// if it was specified in configuration.
type Stage1Data struct {
	Name     string
	Version  string
	Location string
}

// Config is a single place where configuration for rkt frontend needs
// resides.
type Config struct {
	AuthPerHost                  map[string]Headerer
	DockerCredentialsPerRegistry map[string]BasicCredentials
	Paths                        ConfigurablePaths
	Stage1                       Stage1Data
}

func newConfig() *Config {
	return &Config{
		AuthPerHost:                  make(map[string]Headerer),
		DockerCredentialsPerRegistry: make(map[string]BasicCredentials),
	}
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

type authV1JsonParser struct {
	parserBase
}

var _ baseconfig.Parser = (*authV1JsonParser)(nil)
var _ configPropagator = (*authV1JsonParser)(nil)

type dockerAuthV1JsonParser struct {
	parserBase
}

var _ baseconfig.Parser = (*dockerAuthV1JsonParser)(nil)
var _ configPropagator = (*dockerAuthV1JsonParser)(nil)

type pathsV1JsonParser struct {
	parserBase
}

var _ baseconfig.Parser = (*pathsV1JsonParser)(nil)
var _ configPropagator = (*pathsV1JsonParser)(nil)

type stage1V1JsonParser struct {
	parserBase
}

var _ baseconfig.Parser = (*stage1V1JsonParser)(nil)
var _ configPropagator = (*stage1V1JsonParser)(nil)
