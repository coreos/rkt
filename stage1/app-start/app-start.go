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

	rktlog "github.com/coreos/rkt/pkg/log"
	stage1initcommon "github.com/coreos/rkt/stage1/init/common"

	"github.com/appc/spec/schema/types"
)

var (
	debug bool
	log   *rktlog.Logger
	diag  *rktlog.Logger
)

func init() {
	flag.BoolVar(&debug, "debug", false, "Run in debug mode")
}

func main() {
	flag.Parse()

	stage1initcommon.InitDebug(debug)

	log, diag, _ = rktlog.NewLogSet("stage1", debug)
	if !debug {
		diag.SetOutput(ioutil.Discard)
	}

	appName, err := types.NewACName(flag.Arg(1))
	if err != nil {
		log.PrintE("invalid app name", err)
		os.Exit(254)
	}

	enterCmd := []string{flag.Arg(2)}
	enterCmd = append(enterCmd, fmt.Sprintf("--pid=%s", flag.Arg(3)), "--")

	args := enterCmd
	args = append(args, "/usr/bin/systemctl")
	args = append(args, "start")
	args = append(args, appName.String())

	cmd := exec.Cmd{
		Path: args[0],
		Args: args,
	}

	if err := cmd.Run(); err != nil {
		log.PrintE(fmt.Sprintf("error starting app %q", appName.String()), err)
		os.Exit(254)
	}

	os.Exit(0)
}
