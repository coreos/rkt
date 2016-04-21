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

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/appc/cni/pkg/skel"
	"github.com/appc/cni/pkg/types"
)

func main() {
	skel.PluginMain(cmdAdd, cmdDel)
}

type stubConf struct {
	Path string `json:"path"`
	Text string `json:"text"`
}

func cmdAdd(args *skel.CmdArgs) error {
	cfg, err := loadConf(args.StdinData)
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(cfg.Path, []byte(cfg.Text), 0755); err != nil {
		return fmt.Errorf("failed to write %q to file %q: %v", cfg.Text, cfg.Path, err)
	}
	return (&types.Result{}).Print()
}

func cmdDel(args *skel.CmdArgs) error {
	cfg, err := loadConf(args.StdinData)
	if err != nil {
		return err
	}

	return os.Remove(cfg.Path)
}

func loadConf(raw []byte) (*stubConf, error) {
	cfg := &stubConf{}
	if err := json.Unmarshal(raw, cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal the stub config: %v", err)
	}
	if cfg.Path == "" {
		return nil, errors.New(`"path" field is required`)
	}
	if cfg.Text == "" {
		return nil, errors.New(`"text" field is required`)
	}
	return cfg, nil
}
