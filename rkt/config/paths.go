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
	"path/filepath"

	baseconfig "github.com/coreos/rkt/pkg/config"
)

type pathsV1 struct {
	Data         string `json:"data"`
	Stage1Images string `json:"stage1-images"`
}

func (p *pathsV1JsonParser) Parse(idx *baseconfig.PathIndex, raw []byte) error {
	var paths pathsV1
	if err := json.Unmarshal(raw, &paths); err != nil {
		return err
	}
	config := p.getConfig(idx.Index)
	if paths.Data != "" {
		if config.Paths.DataDir != "" {
			return errors.New("data directory is already specified")
		}
		if !filepath.IsAbs(paths.Data) {
			return errors.New("data directory must be an absolute path")
		}
		config.Paths.DataDir = paths.Data
	}
	if paths.Stage1Images != "" {
		if config.Paths.Stage1ImagesDir != "" {
			return errors.New("stage1 images directory is already specified")
		}
		if !filepath.IsAbs(paths.Stage1Images) {
			return errors.New("stage1 images directory must be an absolute path")
		}
		config.Paths.Stage1ImagesDir = paths.Stage1Images
	}

	return nil
}

func (p *pathsV1JsonParser) propagateConfig(config *Config) {
	for _, subconfig := range p.configs {
		if subconfig.Paths.DataDir != "" {
			config.Paths.DataDir = subconfig.Paths.DataDir
		}
		if subconfig.Paths.Stage1ImagesDir != "" {
			config.Paths.Stage1ImagesDir = subconfig.Paths.Stage1ImagesDir
		}
	}
}
