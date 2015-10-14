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
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/spf13/cobra"
	"github.com/coreos/rkt/common"
	"github.com/coreos/rkt/pkg/keystore"
	"github.com/coreos/rkt/pkg/multicall"
	"github.com/coreos/rkt/rkt/config"
)

const (
	cliName        = "rkt"
	cliDescription = "rkt, the application container runner"

	defaultDataDir = "/var/lib/rkt"
)

type absDir struct {
	dir string
}

func (d *absDir) String() string {
	return d.dir
}

func (d *absDir) Set(str string) error {
	if str == "" {
		return fmt.Errorf(`"" is not a valid directory`)
	}

	dir, err := filepath.Abs(str)
	if err != nil {
		return err
	}
	d.dir = dir
	return nil
}

func (d *absDir) Type() string {
	return "absolute-directory"
}

var (
	tabOut      *tabwriter.Writer
	globalFlags = struct {
		Dir                absDir
		SystemConfigDir    absDir
		LocalConfigDir     absDir
		Debug              bool
		Help               bool
		InsecureSkipVerify bool
		TrustKeysFromHttps bool
	}{
		Dir:             absDir{defaultDataDir},
		SystemConfigDir: absDir{common.DefaultSystemConfigDir},
		LocalConfigDir:  absDir{common.DefaultLocalConfigDir},
	}

	cmdExitCode int
)

var cmdRkt = &cobra.Command{
	Use:   "rkt [command]",
	Short: cliDescription,
}

func init() {
	cmdRkt.PersistentFlags().BoolVar(&globalFlags.Debug, "debug", false, "print out more debug information to stderr")
	cmdRkt.PersistentFlags().Var(&globalFlags.Dir, "dir", "rkt data directory")
	cmdRkt.PersistentFlags().Var(&globalFlags.SystemConfigDir, "system-config", "system configuration directory")
	cmdRkt.PersistentFlags().Var(&globalFlags.LocalConfigDir, "local-config", "local configuration directory")
	cmdRkt.PersistentFlags().BoolVar(&globalFlags.InsecureSkipVerify, "insecure-skip-verify", false, "skip all TLS, image or fingerprint verification")
	cmdRkt.PersistentFlags().BoolVar(&globalFlags.TrustKeysFromHttps, "trust-keys-from-https", true, "automatically trust gpg keys fetched from https")
}

func init() {
	cobra.EnablePrefixMatching = true
}

func getTabOutWithWriter(writer io.Writer) *tabwriter.Writer {
	aTabOut := new(tabwriter.Writer)
	aTabOut.Init(writer, 0, 8, 1, '\t', 0)
	return aTabOut
}

// runWrapper return a func(cmd *cobra.Command, args []string) that internally
// will add command function return code and the reinsertion of the "--" flag
// terminator.
func runWrapper(cf func(cmd *cobra.Command, args []string) (exit int)) func(cmd *cobra.Command, args []string) {
	return func(cmd *cobra.Command, args []string) {
		cmdExitCode = cf(cmd, args)
	}
}

func main() {
	// check if rkt is executed with a multicall command
	multicall.MaybeExec()

	cmdRkt.SetUsageFunc(usageFunc)

	// Make help just show the usage
	cmdRkt.SetHelpTemplate(`{{.UsageString}}`)

	cmdRkt.Execute()
	os.Exit(cmdExitCode)
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
	return filepath.Join(globalFlags.Dir.String(), "pods", "embryo")
}

// where pod trees reside during (locked) and after failing to complete preparation (unlocked)
func prepareDir() string {
	return filepath.Join(globalFlags.Dir.String(), "pods", "prepare")
}

// where pod trees reside upon successful preparation
func preparedDir() string {
	return filepath.Join(globalFlags.Dir.String(), "pods", "prepared")
}

// where pod trees reside once run
func runDir() string {
	return filepath.Join(globalFlags.Dir.String(), "pods", "run")
}

// where pod trees reside once exited & marked as garbage by a gc pass
func exitedGarbageDir() string {
	return filepath.Join(globalFlags.Dir.String(), "pods", "exited-garbage")
}

// where never-executed pod trees reside once marked as garbage by a gc pass (failed prepares, expired prepareds)
func garbageDir() string {
	return filepath.Join(globalFlags.Dir.String(), "pods", "garbage")
}

func getKeystore() *keystore.Keystore {
	if globalFlags.InsecureSkipVerify {
		return nil
	}
	config := keystore.NewConfig(globalFlags.SystemConfigDir.String(), globalFlags.LocalConfigDir.String())
	return keystore.New(config)
}

func getConfig() (*config.Config, error) {
	return config.GetConfigFrom(globalFlags.SystemConfigDir.String(), globalFlags.LocalConfigDir.String())
}

func lockDir() string {
	return filepath.Join(globalFlags.Dir.String(), "locks")
}
