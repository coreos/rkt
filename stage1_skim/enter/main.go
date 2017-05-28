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
	"syscall"

	rktlog "github.com/coreos/rkt/pkg/log"
)

var (
	debug   bool
	podPid  string
	appName string

	log  *rktlog.Logger
	diag *rktlog.Logger
)

func init() {
	flag.BoolVar(&debug, "debug", false, "Run in debug mode")
	flag.StringVar(&podPid, "pid", "", "Pod PID (Ignored by updated rkt skim)")
	flag.StringVar(&appName, "appname", "", "Application")

	log, diag, _ = rktlog.NewLogSet("skim-enter", false)
}

func execArgs() error {
	argv0 := flag.Arg(0)
	argv := flag.Args()
	envv := []string{}

	return syscall.Exec(argv0, argv, envv)
}

func main() {
	flag.Parse()

	log.SetDebug(debug)
	diag.SetDebug(debug)

	if !debug {
		diag.SetOutput(ioutil.Discard)
	}

	cwd, err := os.Getwd()
	if err != nil {
		log.FatalE("Failed to get current working directory", err)
	}

	if err := os.Chdir(cwd + "/stage1/rootfs/opt/stage2/" + appName + "/rootfs"); err != nil {
		log.FatalE("Failed to change to new root", err)
	}

	diag.Println("PID:", podPid)
	diag.Println("APP:", appName)
	diag.Println("ARGS:", flag.Args())

	if err := execArgs(); err != nil {
		log.PrintE("exec failed", err)
	}

	os.Exit(254)
}
