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
	"github.com/coreos/rkt/common"
	pkgPod "github.com/coreos/rkt/pkg/pod"
	"github.com/coreos/rkt/stage0"
	"github.com/coreos/rkt/store/imagestore"
	"github.com/coreos/rkt/store/treestore"
	"github.com/spf13/cobra"
)

const (
	cmdRunPreparedName = "run-prepared"
)

var (
	cmdRunPrepared = &cobra.Command{
		Use:   "run-prepared UUID",
		Short: "Run a prepared application pod in rkt",
		Long:  `Runs a previously prepared pod by its UUID`,
		Run:   ensureSuperuser(runWrapper(runRunPrepared)),
	}
)

func init() {
	cmdRkt.AddCommand(cmdRunPrepared)

	cmdRunPrepared.Flags().Var(&flagNet, "net", "configure the pod's networking. Optionally, pass a list of user-configured networks to load and set arguments to pass to each network, respectively. Syntax: --net[=n[:args]][,]")
	cmdRunPrepared.Flags().Lookup("net").NoOptDefVal = "default"
	cmdRunPrepared.Flags().Var(&flagDNS, "dns", "name servers to write in /etc/resolv.conf")
	cmdRunPrepared.Flags().Var(&flagDNSSearch, "dns-search", "DNS search domains to write in /etc/resolv.conf")
	cmdRunPrepared.Flags().Var(&flagDNSOpt, "dns-opt", "DNS options to write in /etc/resolv.conf")
	cmdRunPrepared.Flags().BoolVar(&flagInteractive, "interactive", false, "run pod interactively")
	cmdRunPrepared.Flags().BoolVar(&flagMDSRegister, "mds-register", false, "register pod with metadata service")
	cmdRunPrepared.Flags().StringVar(&flagHostname, "hostname", "", `pod's hostname. If empty, it will be "rkt-$PODUUID"`)
}

func runRunPrepared(cmd *cobra.Command, args []string) (exit int) {
	if len(args) != 1 {
		cmd.Usage()
		return 1
	}

	p, err := pkgPod.PodFromUUIDString(getDataDir(), args[0])
	if err != nil {
		stderr.PrintE("problem retrieving pod", err)
		return 1
	}
	defer p.Close()

	s, err := imagestore.NewStore(storeDir())
	if err != nil {
		stderr.PrintE("cannot open store", err)
		return 1
	}

	ts, err := treestore.NewStore(treeStoreDir(), s)
	if err != nil {
		stderr.PrintE("cannot open treestore", err)
		return 1
	}

	if p.State() != pkgPod.Prepared {
		stderr.Printf("pod %q is not prepared", p.UUID)
		return 1
	}

	_, manifest, err := p.PodManifest()
	if err != nil {
		stderr.PrintE("cannot read pod manifest", err)
		return 1
	}

	if flagInteractive {
		if len(manifest.Apps) > 1 {
			stderr.Print("interactive option only supports pods with one app")
			return 1
		}
	}

	// Make sure we have a metadata service available before we move to
	// run state so that the user can rerun the command without needing
	// to prepare the image again.
	if flagMDSRegister {
		if err := stage0.CheckMdsAvailability(); err != nil {
			stderr.Error(err)
			return 1
		}
	}

	if err := p.ToRun(); err != nil {
		stderr.PrintE("cannot transition to run", err)
		return 1
	}

	lfd, err := p.Fd()
	if err != nil {
		stderr.PrintE("unable to get lock fd", err)
		return 1
	}

	rktgid, err := common.LookupGid(common.RktGroup)
	if err != nil {
		stderr.Printf("group %q not found, will use default gid when rendering images", common.RktGroup)
		rktgid = -1
	}

	ovlOk := true
	if err := common.PathSupportsOverlay(getDataDir()); err != nil {
		if oerr, ok := err.(common.ErrOverlayUnsupported); ok {
			stderr.Printf("disabling overlay support: %q", oerr.Error())
			ovlOk = false
		} else {
			stderr.PrintE("error determining overlay support", err)
			return 1
		}
	}

	ovlPrep := p.UsesOverlay()

	// should not happen, maybe the data directory moved from an overlay-enabled fs to another location
	// between prepare and run-prepared
	if ovlPrep && !ovlOk {
		stderr.Print("unable to run prepared overlay-enabled pod: overlay not supported")
		return 1
	}

	rcfg := stage0.RunConfig{
		CommonConfig: &stage0.CommonConfig{
			Store:     s,
			TreeStore: ts,
			UUID:      p.UUID,
			Debug:     globalFlags.Debug,
		},
		Net:                  flagNet,
		LockFd:               lfd,
		Interactive:          flagInteractive,
		DNS:                  flagDNS,
		DNSSearch:            flagDNSSearch,
		DNSOpt:               flagDNSOpt,
		MDSRegister:          flagMDSRegister,
		Apps:                 manifest.Apps,
		RktGid:               rktgid,
		Hostname:             flagHostname,
		InsecureCapabilities: globalFlags.InsecureFlags.SkipCapabilities(),
		InsecurePaths:        globalFlags.InsecureFlags.SkipPaths(),
		InsecureSeccomp:      globalFlags.InsecureFlags.SkipSeccomp(),
		UseOverlay:           ovlPrep && ovlOk,
	}
	if globalFlags.Debug {
		stage0.InitDebug()
	}
	stage0.Run(rcfg, p.Path(), getDataDir()) // execs, never returns
	return 1
}
