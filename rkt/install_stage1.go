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
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/coreos/rkt/common/apps"
	"github.com/coreos/rkt/rkt/image"
	"github.com/coreos/rkt/store"
	"github.com/spf13/cobra"
)

var (
	cmdInstallStage1 = &cobra.Command{
		Use:   "stage1 LOCATION",
		Short: "Fetches a stage1 image file into the store and writes a stage1 configuration file in the vendor directory",
		Long:  "This command should be executed when installing rkt in the filesystem. It will make sure that the vendor default stage image1 is in the store, so commands like run or prepare will be will be faster. The location passed to this command should be either an absolute path or an HTTP/HTTPS/File/Docker URL",
		Run:   runWrapper(runInstallStage1),
	}
)

const (
	stage1ConfigTemplate = `
{
	"rktKind": "stage1",
	"rktVersion": "v1",
	"name": "^NAME^",
	"version": "^VERSION^",
	"location": "^LOCATION^"
}
`
)

func init() {
	cmdInstall.AddCommand(cmdInstallStage1)
}

func runInstallStage1(cmd *cobra.Command, args []string) int {
	if len(args) != 1 {
		stderr.Printf("must provide exactly one absolute path or URL to the stage1 image")
		return 1
	}
	locType := apps.AppImageGuess
	loc := args[0]
	if filepath.IsAbs(loc) {
		locType = apps.AppImagePath
	} else {
		u, err := url.Parse(loc)
		if err != nil {
			stderr.PrintE(fmt.Sprintf("location %q is neither an absolute path nor a valid URL", loc), err)
			return 1
		}
		if u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "docker" && u.Scheme != "file" {
			stderr.Printf("URL %q has an invalid scheme", loc)
			return 1
		}
		locType = apps.AppImageURL
	}
	s, err := store.NewStore(getDataDir())
	if err != nil {
		stderr.PrintE("cannot open store", err)
		return 1
	}
	ks := getKeystore()
	config, err := getConfig()
	if err != nil {
		stderr.PrintE("cannot get configuration", err)
		return 1
	}

	ft := &image.Fetcher{
		S:                  s,
		Ks:                 ks,
		Headers:            config.AuthPerHost,
		DockerAuth:         config.DockerCredentialsPerRegistry,
		InsecureFlags:      globalFlags.InsecureFlags,
		Debug:              globalFlags.Debug,
		TrustKeysFromHTTPS: globalFlags.TrustKeysFromHTTPS,

		StoreOnly: false,
		NoStore:   false,
		WithDeps:  true,
	}

	hash, err := ft.FetchImage(loc, "", locType)
	if err != nil {
		stderr.PrintE(fmt.Sprintf("cannot fetch stage1 image %q", loc), err)
		return 1
	}
	im, err := s.GetImageManifest(hash)
	if err != nil {
		stderr.PrintE(fmt.Sprintf("cannot get the stage1 image's (%q, hash %q) manifest from store", loc, hash), err)
		return 1
	}
	version, ok := im.GetLabel("version")
	if !ok {
		stderr.Printf("the stage1 image file %q has no version", loc)
		return 1
	}
	cfg := stage1ConfigTemplate
	cfg = strings.Replace(cfg, "^NAME^", im.Name.String(), 1)
	cfg = strings.Replace(cfg, "^VERSION^", version, 1)
	cfg = strings.Replace(cfg, "^LOCATION^", loc, 1)

	cfgDir := filepath.Join(globalFlags.SystemConfigDir, "stage1.d")
	cfgPath := filepath.Join(cfgDir, "vendor-stage1.json")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		stderr.PrintE(fmt.Sprintf("failed to create directories %q", cfgDir), err)
		return 1
	}
	if err := ioutil.WriteFile(cfgPath, []byte(cfg), 0755); err != nil {
		stderr.PrintE(fmt.Sprintf("failed to write a vendor configuration to %q", cfgPath), err)
		return 1
	}

	return 0
}
