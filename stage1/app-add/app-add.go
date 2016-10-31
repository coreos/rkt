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

//+build linux

package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/coreos/go-systemd/unit"
	"github.com/coreos/rkt/common/cgroup"
	"github.com/coreos/rkt/common/cgroup/v1"
	rktlog "github.com/coreos/rkt/pkg/log"
	stage1types "github.com/coreos/rkt/stage1/common/types"
	stage1initcommon "github.com/coreos/rkt/stage1/init/common"

	"github.com/appc/spec/schema/types"
)

var (
	debug               bool
	disableCapabilities bool
	disablePaths        bool
	disableSeccomp      bool
	privateUsers        string
	log                 *rktlog.Logger
	diag                *rktlog.Logger
)

func init() {
	flag.BoolVar(&debug, "debug", false, "Run in debug mode")
	flag.BoolVar(&disableCapabilities, "disable-capabilities-restriction", false, "Disable capability restrictions")
	flag.BoolVar(&disablePaths, "disable-paths", false, "Disable paths restrictions")
	flag.BoolVar(&disableSeccomp, "disable-seccomp", false, "Disable seccomp restrictions")
	flag.StringVar(&privateUsers, "private-users", "", "Run within user namespace. Can be set to [=UIDBASE[:NUIDS]]")
}

// TODO use named flags instead of positional
func main() {
	flag.Parse()

	stage1initcommon.InitDebug(debug)

	log, diag, _ = rktlog.NewLogSet("stage1", debug)
	if !debug {
		diag.SetOutput(ioutil.Discard)
	}

	uuid, err := types.NewUUID(flag.Arg(0))
	if err != nil {
		log.PrintE("UUID is missing or malformed", err)
		os.Exit(254)
	}

	appName, err := types.NewACName(flag.Arg(1))
	if err != nil {
		log.PrintE("invalid app name", err)
		os.Exit(254)
	}

	enterCmd := []string{flag.Arg(2)}
	enterCmd = append(enterCmd, fmt.Sprintf("--pid=%s", flag.Arg(3)), "--")

	root := "."
	p, err := stage1types.LoadPod(root, uuid)
	if err != nil {
		log.PrintE("failed to load pod", err)
		os.Exit(254)
	}

	insecureOptions := stage1initcommon.Stage1InsecureOptions{
		DisablePaths:        disablePaths,
		DisableCapabilities: disableCapabilities,
		DisableSeccomp:      disableSeccomp,
	}

	ra := p.Manifest.Apps.Get(*appName)
	if ra == nil {
		log.Printf("failed to get app")
		os.Exit(254)
	}

	if ra.App.WorkingDirectory == "" {
		ra.App.WorkingDirectory = "/"
	}

	/* prepare cgroups */
	isUnified, err := cgroup.IsCgroupUnified("/")
	if err != nil {
		log.FatalE("failed to determine the cgroup version", err)
		os.Exit(254)
	}

	if !isUnified {
		enabledCgroups, err := v1.GetEnabledCgroups()
		if err != nil {
			log.FatalE("error getting cgroups", err)
			os.Exit(254)
		}

		b, err := ioutil.ReadFile(filepath.Join(p.Root, "subcgroup"))
		if err == nil {
			subcgroup := string(b)
			serviceName := stage1initcommon.ServiceUnitName(ra.Name)

			if err := v1.RemountCgroupKnobsRW(enabledCgroups, subcgroup, serviceName, enterCmd); err != nil {
				log.FatalE("error restricting container cgroups", err)
				os.Exit(254)
			}
		} else {
			log.PrintE("continuing with per-app isolators disabled", err)
		}
	}

	stage1initcommon.AppAddMounts(p, ra, enterCmd)

	/* write service file */
	binPath, err := stage1initcommon.FindBinPath(p, ra)
	if err != nil {
		log.PrintE("failed to find bin path", err)
		os.Exit(254)
	}

	w := stage1initcommon.NewUnitWriter(p)

	w.AppUnit(ra, binPath, privateUsers, insecureOptions,
		unit.NewUnitOption("Unit", "Before", "halt.target"),
		unit.NewUnitOption("Unit", "Conflicts", "halt.target"),
		unit.NewUnitOption("Service", "StandardOutput", "journal+console"),
		unit.NewUnitOption("Service", "StandardError", "journal+console"),
	)

	w.AppReaperUnit(ra.Name, binPath)

	if err := w.Error(); err != nil {
		log.PrintE("error generating app units", err)
		os.Exit(254)
	}

	args := enterCmd
	args = append(args, "/usr/bin/systemctl")
	args = append(args, "daemon-reload")

	cmd := exec.Cmd{
		Path: args[0],
		Args: args,
	}

	if err := cmd.Run(); err != nil {
		log.PrintE(`error executing "systemctl daemon-reload"`, err)
		os.Exit(254)
	}

	os.Exit(0)
}
