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

//+build linux

package main

import (
	"io/ioutil"
	"log"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/spf13/cobra"
	"github.com/coreos/rkt/stage0"
	"github.com/coreos/rkt/store"
)

const (
	cmdRunPreparedName = "run-prepared"
)

var (
	cmdRunPrepared = &cobra.Command{
		Use:   "run-prepared UUID",
		Short: "Run a prepared application pod in rkt",
		Long:  "UUID must have been acquired via `rkt prepare`",
		Run:   runWrapper(runRunPrepared),
	}
)

func init() {
	cmdRkt.AddCommand(cmdRunPrepared)

	cmdRunPrepared.Flags().Var(&flagPrivateNet, "private-net", "give pod a private network")
	cmdRunPrepared.Flags().Lookup("private-net").NoOptDefVal = "all"
	cmdRunPrepared.Flags().BoolVar(&flagInteractive, "interactive", false, "the pod is interactive")
	cmdRunPrepared.Flags().BoolVar(&flagMDSRegister, "mds-register", true, "register pod with metadata service")
}

func runRunPrepared(cmd *cobra.Command, args []string) (exit int) {
	if len(args) != 1 {
		cmd.Usage()
		return 1
	}

	podUUID, err := resolveUUID(args[0])
	if err != nil {
		stderr("Unable to resolve UUID: %v", err)
		return 1
	}

	dataDir := getDataDir()
	if dataDir == "" {
		log.Printf("dir unset - using temporary directory")
		var err error
		dataDir, err = ioutil.TempDir("", "rkt")
		if err != nil {
			stderr("error creating temporary directory: %v", err)
			return 1
		}
	}

	s, err := store.NewStore(dataDir)
	if err != nil {
		stderr("prepared-run: cannot open store: %v", err)
		return 1
	}

	p, err := getPod(podUUID)
	if err != nil {
		stderr("prepared-run: cannot get pod: %v", err)
		return 1
	}

	if !p.isPrepared {
		stderr("prepared-run: pod %q is not prepared", podUUID.String())
		return 1
	}

	if flagInteractive {
		ac, err := p.getAppCount()
		if err != nil {
			stderr("prepared-run: cannot get pod's app count: %v", err)
			return 1
		}
		if ac > 1 {
			stderr("prepared-run: interactive option only supports pods with one app")
			return 1
		}
	}

	if err := p.xToRun(); err != nil {
		stderr("prepared-run: cannot transition to run: %v", err)
		return 1
	}

	lfd, err := p.Fd()
	if err != nil {
		stderr("prepared-run: unable to get lock fd: %v", err)
		return 1
	}

	s1img, err := p.getStage1Hash()
	if err != nil {
		stderr("prepared-run: unable to get stage1 Hash: %v", err)
		return 1
	}

	apps, err := p.getApps()
	if err != nil {
		stderr("prepared-run: unable to get app list: %v", err)
		return 1
	}

	rcfg := stage0.RunConfig{
		CommonConfig: stage0.CommonConfig{
			Store:       s,
			Stage1Image: *s1img,
			UUID:        p.uuid,
			Debug:       globalFlags.Debug,
		},
		PrivateNet:  flagPrivateNet,
		LockFd:      lfd,
		Interactive: flagInteractive,
		MDSRegister: flagMDSRegister,
		Apps:        apps,
	}
	stage0.Run(rcfg, p.path()) // execs, never returns
	return 1
}
