// Copyright 2014 The rkt Authors
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
	"flag"
	"fmt"
	"os"
	"runtime"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/schema/types"
	"github.com/coreos/rkt/common/apps"
	"github.com/coreos/rkt/store"
)

const (
	defaultOS   = runtime.GOOS
	defaultArch = runtime.GOARCH

	defaultPathPerm os.FileMode = 0777
)

var (
	cmdFetch = &Command{
		Name:                 "fetch",
		Summary:              "Fetch image(s) and store them in the local cache",
		Usage:                "IMAGE_URL...",
		Run:                  runFetch,
		Flags:                &fetchFlags,
		WantsFlagsTerminator: true,
	}
	fetchFlags flag.FlagSet
)

func init() {
	commands = append(commands, cmdFetch)
	fetchFlags.Var((*appAsc)(&rktApps), "signature", "local signature file to use in validating the preceding image")
}

func runFetch(args []string) (exit int) {
	if err := parseApps(&rktApps, args, &fetchFlags, false); err != nil {
		stderr("fetch: unable to parse arguments: %v", err)
		return 1
	}
	if rktApps.Count() < 1 {
		stderr("fetch: must provide at least one image")
		return 1
	}

	s, err := store.NewStore(globalFlags.Dir)
	if err != nil {
		stderr("fetch: cannot open store: %v", err)
		return 1
	}
	ks := getKeystore()
	config, err := getConfig()
	if err != nil {
		stderr("fetch: cannot get configuration: %v", err)
		return 1
	}
	ft := &fetcher{
		imageActionData: imageActionData{
			s:                  s,
			ks:                 ks,
			headers:            config.AuthPerHost,
			dockerAuth:         config.DockerCredentialsPerRegistry,
			insecureSkipVerify: globalFlags.InsecureSkipVerify,
			debug:              globalFlags.Debug,
		},
		withDeps: true,
	}

	err = rktApps.Walk(func(app *apps.App) error {
		hash, err := ft.fetchImage(app.Image, app.Asc, true)
		if err != nil {
			return err
		}
		shortHash := types.ShortHash(hash)
		fmt.Println(shortHash)
		return nil
	})
	if err != nil {
		stderr("%v", err)
		return 1
	}

	return
}
