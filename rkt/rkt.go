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
	"fmt"
	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/spf13/cobra"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/coreos/rkt/common"
	"github.com/coreos/rkt/pkg/keystore"
	"github.com/coreos/rkt/rkt/config"
)

const (
	cliName        = "rkt"
	cliDescription = "rkt, the application container runner"

	defaultDataDir = "/var/lib/rkt"
)

var (
	rktCmd = &cobra.Command{
		Use:   cliName,
		Short: cliDescription,
	}
	flagDebug              bool
	flagDataDir            string
	flagSystemConfigDir    string
	flagLocalConfigDir     string
	flagInsecureSkipVerify bool
	subCmdExitCode         int

	//Global-ish
	flagImage     = NewMulti("image")
	flagSign      = NewBuddy(flagImage, "signature")
	flagImageArgs = NewBuddy(flagImage, "image-args")

	tabOut *tabwriter.Writer
)

func init() {
	rktCmd.PersistentFlags().BoolVarP(&flagDebug, "debug", "", false, "Print out more debug information to stderr")
	rktCmd.PersistentFlags().StringVarP(&flagDataDir, "dir", "", defaultDataDir, "rkt data directory")
	rktCmd.PersistentFlags().StringVarP(&flagSystemConfigDir, "system-config", "", common.DefaultSystemConfigDir, "system configuration directory")
	rktCmd.PersistentFlags().StringVarP(&flagLocalConfigDir, "local-config", "", common.DefaultLocalConfigDir, "local configuration directory")
	rktCmd.PersistentFlags().BoolVarP(&flagInsecureSkipVerify, "insecure-skip-verify", "", false, "skip image or key verification")
}

func init() {
	tabOut = new(tabwriter.Writer)
	tabOut.Init(os.Stdout, 0, 8, 1, '\t', 0)
}

func main() {
	if !flagDebug {
		log.SetOutput(ioutil.Discard)
	}
	rktCmd.Execute()
	os.Exit(subCmdExitCode)
}

func stderr(format string, a ...interface{}) {
	out := fmt.Sprintf(format, a...)
	fmt.Fprintln(os.Stderr, strings.TrimSuffix(out, "\n"))
}

func stdout(format string, a ...interface{}) {
	out := fmt.Sprintf(format, a...)
	fmt.Fprintln(os.Stdout, strings.TrimSuffix(out, "\n"))
}

// where pod directories are created and locked before moving to prepared
func embryoDir() string {
	return filepath.Join(flagDataDir, "pods", "embryo")
}

// where pod trees reside during (locked) and after failing to complete preparation (unlocked)
func prepareDir() string {
	return filepath.Join(flagDataDir, "pods", "prepare")
}

// where pod trees reside upon successful preparation
func preparedDir() string {
	return filepath.Join(flagDataDir, "pods", "prepared")
}

// where pod trees reside once run
func runDir() string {
	return filepath.Join(flagDataDir, "pods", "run")
}

// where pod trees reside once exited & marked as garbage by a gc pass
func exitedGarbageDir() string {
	return filepath.Join(flagDataDir, "pods", "exited-garbage")
}

// where never-executed pod trees reside once marked as garbage by a gc pass (failed prepares, expired prepareds)
func garbageDir() string {
	return filepath.Join(flagDataDir, "pods", "garbage")
}

func getKeystore() *keystore.Keystore {
	if flagInsecureSkipVerify {
		return nil
	}
	config := keystore.NewConfig(flagSystemConfigDir, flagLocalConfigDir)
	return keystore.New(config)
}

func getConfig() (*config.Config, error) {
	return config.GetConfigFrom(flagSystemConfigDir, flagLocalConfigDir)
}
