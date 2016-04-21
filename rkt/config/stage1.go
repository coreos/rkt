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
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"strings"

	baseconfig "github.com/coreos/rkt/pkg/config"
	"github.com/hashicorp/errwrap"
)

type stage1V1 struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	Location string `json:"location"`
}

var (
	allowedSchemes = map[string]struct{}{
		"file":   struct{}{},
		"docker": struct{}{},
		"http":   struct{}{},
		"https":  struct{}{},
	}
)

func (p *stage1V1JsonParser) Parse(idx *baseconfig.PathIndex, raw []byte) error {
	var stage1 stage1V1
	if err := json.Unmarshal(raw, &stage1); err != nil {
		return err
	}
	if err := p.validateStage1V1(&stage1); err != nil {
		return errwrap.Wrap(errors.New("invalid stage1 configuration"), err)
	}
	config := p.getConfig(idx.Index)
	// At this point either both name and version are specified or
	// neither. The same goes for data in Config.
	if stage1.Name != "" {
		if config.Stage1.Name != "" {
			return errors.New("name and version of a default stage1 image are already specified")
		}
		config.Stage1.Name = stage1.Name
		config.Stage1.Version = stage1.Version
	}
	if stage1.Location != "" {
		if config.Stage1.Location != "" {
			return errors.New("location of a default stage1 image is already specified")
		}
		config.Stage1.Location = stage1.Location
	}
	return nil
}

func (p *stage1V1JsonParser) validateStage1V1(stage1 *stage1V1) error {
	if stage1.Name == "" && stage1.Version != "" {
		return errors.New("default stage1 image version specified, but name is missing")
	}
	if stage1.Name != "" && stage1.Version == "" {
		return errors.New("default stage1 image name specified, but version is missing")
	}
	if stage1.Location != "" {
		if !filepath.IsAbs(stage1.Location) {
			url, err := url.Parse(stage1.Location)
			if err != nil {
				return errwrap.Wrap(errors.New("default stage1 image location is an invalid URL"), err)
			}
			if url.Scheme == "" {
				return errors.New("default stage1 image location is either a relative path or a URL without scheme")
			}
			if _, ok := allowedSchemes[url.Scheme]; !ok {
				schemes := make([]string, 0, len(allowedSchemes))
				for k := range allowedSchemes {
					schemes = append(schemes, fmt.Sprintf("%q", k))
				}
				sort.Strings(schemes)
				return fmt.Errorf("default stage1 image location URL has invalid scheme %q, allowed schemes are %s", url.Scheme, strings.Join(schemes, ", "))
			}
		}
	}
	return nil
}

func (p *stage1V1JsonParser) propagateConfig(config *Config) {
	for _, subconfig := range p.configs {
		// At this point either both name and version are
		// specified or neither.
		if subconfig.Stage1.Name != "" {
			config.Stage1.Name = subconfig.Stage1.Name
			config.Stage1.Version = subconfig.Stage1.Version
		}
		if subconfig.Stage1.Location != "" {
			config.Stage1.Location = subconfig.Stage1.Location
		}
	}
}
