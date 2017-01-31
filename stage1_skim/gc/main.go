// Copyright 2017 The rkt Authors
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
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/appc/spec/schema/types"
	"github.com/coreos/rkt/common"
	stage1init "github.com/coreos/rkt/stage1/init/common"
	rktlog "github.com/coreos/rkt/pkg/log"
)

const (
	systemdPath = "/run/systemd"
)

var (
	debug       bool

	log         *rktlog.Logger
	diag        *rktlog.Logger
	localConfig string
)

func init() {
	flag.BoolVar(&debug, "debug", false, "Run in debug mode")
	flag.StringVar(&localConfig, "local-config", common.DefaultLocalConfigDir, "Local config path (ignored)")
}

func main() {
	flag.Parse()

	log, diag, _ = rktlog.NewLogSet("gc", debug)
	if !debug {
		diag.SetOutput(ioutil.Discard)
	}

	diag.Println("remove all dynamically generated systemd service/slice/scope files")
	podID, err := types.NewUUID(flag.Arg(0))
	if err != nil {
		log.Fatal("UUID is missing or malformed")
	}

	podBase := "rkt-" + podID.String()

	diag.Println("removing transient/*.scope")
	transBase := filepath.Join(systemdPath, "transient")
	transDir, err := ioutil.ReadDir(transBase); if err != nil {
		log.FatalE("Unable to read transient dir", err)
	}

	for _, f := range transDir {
		absFile := filepath.Join(transBase, f.Name())
		if f.Name() == podBase + ".scope" {
			diag.Println("Purging scope: " + absFile)
			os.Remove(absFile)
		}
	}

	diag.Println("removing system/*.[service|slice]")
	sliceName := "system-" + stage1init.SystemdSanitizeSlice(podBase) + ".slice"
	systemBase := filepath.Join(systemdPath, "system")
	systemDir, err := ioutil.ReadDir(systemBase); if err != nil {
		log.FatalE("Unable to read system dir", err)
	}

	for _, f := range systemDir {
		absFile := filepath.Join(systemBase, f.Name())
		if strings.HasSuffix(f.Name(), podBase + ".service") {
			diag.Println("Purging service: " + absFile)
			os.Remove(absFile)
		} else if f.Name() == sliceName {
			diag.Println("Purging slice: " + sliceName)
			os.Remove(absFile)
		}
	}

	diag.Println("reload systemd daemon")
	// reload the systemd's world of unit files
	reloadCmd := exec.Command("/usr/bin/systemctl", "daemon-reload")
	err = reloadCmd.Run(); if err != nil {
		log.FatalE("cannot reload system daemon: ", err)
	}
}
