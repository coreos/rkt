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
	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/spf13/cobra"
	"github.com/coreos/rkt/common"
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

	cmdRunPrepared.Flags().Var(&flagNet, "net", "configure the pod's networking. optionally pass a list of user-configured networks to load and arguments to pass to them. syntax: --net[=n[:args]][,]")
	cmdRunPrepared.Flags().Lookup("net").NoOptDefVal = "default"
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

	s, err := store.NewStore(globalFlags.Dir.String())
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

	// Make sure we have a metadata service available before we move to
	// run state so that the user can rerun the command without needing
	// to prepare the image again.
	if flagMDSRegister {
		if err := stage0.CheckMdsAvailability(); err != nil {
			stderr("prepare-run: %v", err)
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

	apps, err := p.getApps()
	if err != nil {
		stderr("prepared-run: unable to get app list: %v", err)
		return 1
	}

	rktgid, err := common.LookupGid(common.RktGroup)
	if err != nil {
		stderr("prepared-run: group %q not found, will use default gid when rendering images", common.RktGroup)
		rktgid = -1
	}

	rcfg := stage0.RunConfig{
		CommonConfig: stage0.CommonConfig{
			Store: s,
			UUID:  p.uuid,
			Debug: globalFlags.Debug,
		},
		Net:         flagNet,
		LockFd:      lfd,
		Interactive: flagInteractive,
		MDSRegister: flagMDSRegister,
		Apps:        apps,
		RktGid:      rktgid,
	}
	if globalFlags.Debug {
		stage0.InitDebug()
	}
	stage0.Run(rcfg, p.path(), globalFlags.Dir.String()) // execs, never returns
	return 1
}
