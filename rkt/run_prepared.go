// Copyright 2015 CoreOS, Inc.
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

//+build linux

package main

import (
	"flag"
	"io/ioutil"
	"log"

	"github.com/coreos/rocket/cas"
	"github.com/coreos/rocket/stage0"
)

const (
	cmdRunPreparedName = "run-prepared"
)

var (
	cmdRunPrepared = &Command{
		Name:        cmdRunPreparedName,
		Summary:     "Run a prepared application container in rocket",
		Usage:       "UUID",
		Description: "UUID must have been acquired via `rkt prepare`",
		Run:         runRunPrepared,
		Flags:       &runPreparedFlags,
	}
	runPreparedFlags flag.FlagSet
)

func init() {
	commands = append(commands, cmdRunPrepared)
	runPreparedFlags.BoolVar(&flagPrivateNet, "private-net", false, "give container a private network")
	runPreparedFlags.BoolVar(&flagSpawnMetadataService, "spawn-metadata-svc", false, "launch metadata svc if not running")
	runPreparedFlags.BoolVar(&flagInteractive, "interactive", false, "the container is interactive")
}

func runRunPrepared(args []string) (exit int) {
	if len(args) != 1 {
		printCommandUsageByName(cmdRunPreparedName)
		return 1
	}

	containerUUID, err := resolveUUID(args[0])
	if err != nil {
		stderr("Unable to resolve UUID: %v", err)
		return 1
	}

	if globalFlags.Dir == "" {
		log.Printf("dir unset - using temporary directory")
		var err error
		globalFlags.Dir, err = ioutil.TempDir("", "rkt")
		if err != nil {
			stderr("error creating temporary directory: %v", err)
			return 1
		}
	}

	ds, err := cas.NewStore(globalFlags.Dir)
	if err != nil {
		stderr("prepared-run: cannot open store: %v", err)
		return 1
	}

	c, err := getContainer(containerUUID.String())
	if err != nil {
		stderr("prepared-run: cannot get container: %v", err)
		return 1
	}

	if !c.isPrepared {
		stderr("prepared-run: container %q is not prepared", containerUUID.String())
		return 1
	}

	if flagInteractive {
		ac, err := c.getAppCount()
		if err != nil {
			stderr("prepared-run: cannot get container's app count: %v", err)
			return 1
		}
		if ac > 1 {
			stderr("prepared-run: interactive option only supports containers with one app")
			return 1
		}
	}

	if err := c.xToRun(); err != nil {
		stderr("prepared-run: cannot transition to run: %v", err)
		return 1
	}

	lfd, err := c.Fd()
	if err != nil {
		stderr("prepared-run: unable to get lock fd: %v", err)
		return 1
	}

	s1img, err := c.getStage1Hash()
	if err != nil {
		stderr("prepared-run: unable to get stage1 Hash: %v", err)
		return 1
	}

	imgs, err := c.getAppsHashes()
	if err != nil {
		stderr("prepared-run: unable to get apps hashes: %v", err)
		return 1
	}

	rcfg := stage0.RunConfig{
		CommonConfig: stage0.CommonConfig{
			Store:       ds,
			Stage1Image: *s1img,
			UUID:        c.uuid,
			Images:      imgs,
			Debug:       globalFlags.Debug,
		},
		PrivateNet:           flagPrivateNet,
		SpawnMetadataService: flagSpawnMetadataService,
		LockFd:               lfd,
		Interactive:          flagInteractive,
	}
	stage0.Run(rcfg, c.path()) // execs, never returns
	return 1
}
