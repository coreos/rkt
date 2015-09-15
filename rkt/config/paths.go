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
	"fmt"
)

type configurablePathsV1 struct {
	Data string `json:"data"`
}

func init() {
	addParser("paths", "v1", &configurablePathsV1{})
	// Look in 'paths.d' subdir for configs of type paths
	registerSubDir("paths.d", []string{"paths"})
}

func (p *configurablePathsV1) parse(config *Config, raw []byte) error {
	var dirs configurablePathsV1
	if err := json.Unmarshal(raw, &dirs); err != nil {
		return err
	}
	if dirs.Data != "" {
		if config.Paths.DataDir != "" {
			// A clash has occurred. Data dir has been defined more than once in
			// the same directory
			return fmt.Errorf("Error processing the data directory. Verify that it has not been defined more than once in the same config directory.")
		}
		config.Paths.DataDir = dirs.Data
	}

	return nil
}
