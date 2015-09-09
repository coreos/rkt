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
	"strconv"
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

const (
	insecureNone  = 0
	insecureImage = 1 << (iota - 1)
	insecureTls
	insecureOnDisk

	insecureAll = (insecureImage | insecureTls | insecureOnDisk)
)

var (
	insecureOptions = []string{"none", "image", "tls", "ondisk", "all"}

	insecureOptionsMap = map[string]int{
		insecureOptions[0]: insecureNone,
		insecureOptions[1]: insecureImage,
		insecureOptions[2]: insecureTls,
		insecureOptions[3]: insecureOnDisk,
		insecureOptions[4]: insecureAll,
	}
)

// flagInsecureSkipVerify is a deprecated flag that is equivalent to setting
// "--insecure-options" to 'all' when true and 'none' when false.
// TODO: Remove before 1.0 release
type skipVerify struct{}

func (sv *skipVerify) Set(s string) error {
	v, err := strconv.ParseBool(s)
	if err != nil {
		return err
	}

	switch v {
	case false:
		err = (*bitFlags)(globalFlags.InsecureFlags).Set("none")
	case true:
		err = (*bitFlags)(globalFlags.InsecureFlags).Set("all")
	}
	return err
}

func (sv *skipVerify) String() string {
	return fmt.Sprintf("%v", *sv)
}

func (sv *skipVerify) Type() string {
	// Must return "bool" in order to place naked flag before subcommand.
	// For example, rkt --insecure-skip-verify run docker://image
	return "bool"
}

type absDir string

func (d *absDir) String() string {
	return (string)(*d)
}

func (d *absDir) Set(str string) error {
	if str == "" {
		return fmt.Errorf(`"" is not a valid directory`)
	}

	dir, err := filepath.Abs(str)
	if err != nil {
		return err
	}
	*d = (absDir)(dir)
	return nil
}

func (d *absDir) Type() string {
	return "absolute-directory"
}

type secFlags bitFlags

func (sf *secFlags) SkipImageCheck() bool {
	return (*bitFlags)(sf).hasFlag(insecureImage)
}

func (sf *secFlags) SkipTlsCheck() bool {
	return (*bitFlags)(sf).hasFlag(insecureTls)
}

func (sf *secFlags) SkipOnDiskCheck() bool {
	return (*bitFlags)(sf).hasFlag(insecureOnDisk)
}

func (sf *secFlags) SkipAllSecurityChecks() bool {
	return (*bitFlags)(sf).hasFlag(insecureAll)
}

func (sf *secFlags) SkipAnySecurityChecks() bool {
	return (*bitFlags)(sf).flags != 0
}

var (
	tabOut      *tabwriter.Writer
	globalFlags = struct {
		Dir                string
		SystemConfigDir    string
		LocalConfigDir     string
		Debug              bool
		Help               bool
		InsecureFlags      *secFlags
		TrustKeysFromHttps bool
	}{
		Dir:             defaultDataDir,
		SystemConfigDir: common.DefaultSystemConfigDir,
		LocalConfigDir:  common.DefaultLocalConfigDir,
	}

	cmdExitCode int
)

var cmdRkt = &cobra.Command{
	Use:   "rkt [command]",
	Short: cliDescription,
}

func init() {
	bf, err := newBitFlags(insecureOptions, "none", insecureOptionsMap)
	if err != nil {
		stderr("rkt: problem initializing: %v", err)
		os.Exit(1)
	}

	globalFlags.InsecureFlags = (*secFlags)(bf)
	skipVerify := new(skipVerify)

	cmdRkt.PersistentFlags().BoolVar(&globalFlags.Debug, "debug", false, "print out more debug information to stderr")
	cmdRkt.PersistentFlags().Var((*absDir)(&globalFlags.Dir), "dir", "rkt data directory")
	cmdRkt.PersistentFlags().Var((*absDir)(&globalFlags.SystemConfigDir), "system-config", "system configuration directory")
	cmdRkt.PersistentFlags().Var((*absDir)(&globalFlags.LocalConfigDir), "local-config", "local configuration directory")
	cmdRkt.PersistentFlags().Var((*bitFlags)(globalFlags.InsecureFlags), "insecure-options",
		fmt.Sprintf("comma-separated list of security features to disable. Allowed values: %s",
			globalFlags.InsecureFlags.PermissibleString()))
	cmdRkt.PersistentFlags().BoolVar(&globalFlags.TrustKeysFromHttps, "trust-keys-from-https",
		true, "automatically trust gpg keys fetched from https")

	// TODO: Remove before 1.0
	svFlag := cmdRkt.PersistentFlags().VarPF(skipVerify, "insecure-skip-verify", "", "DEPRECATED")
	svFlag.DefValue = "false"
	svFlag.NoOptDefVal = "true"
	svFlag.Hidden = true
	svFlag.Deprecated = "please use --insecure-options."
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

// where rkt fly trees will live
func flightDir() string {
	return filepath.Join(globalFlags.Dir, "flights")
}

// where pod directories are created and locked before moving to prepared
func embryoDir() string {
	return filepath.Join(globalFlags.Dir, "pods", "embryo")
}

// where pod trees reside during (locked) and after failing to complete preparation (unlocked)
func prepareDir() string {
	return filepath.Join(globalFlags.Dir, "pods", "prepare")
}

// where pod trees reside upon successful preparation
func preparedDir() string {
	return filepath.Join(globalFlags.Dir, "pods", "prepared")
}

// where pod trees reside once run
func runDir() string {
	return filepath.Join(globalFlags.Dir, "pods", "run")
}

// where pod trees reside once exited & marked as garbage by a gc pass
func exitedGarbageDir() string {
	return filepath.Join(globalFlags.Dir, "pods", "exited-garbage")
}

// where never-executed pod trees reside once marked as garbage by a gc pass (failed prepares, expired prepareds)
func garbageDir() string {
	return filepath.Join(globalFlags.Dir, "pods", "garbage")
}

func getKeystore() *keystore.Keystore {
	if globalFlags.InsecureFlags.SkipImageCheck() {
		return nil
	}
	config := keystore.NewConfig(globalFlags.SystemConfigDir, globalFlags.LocalConfigDir)
	return keystore.New(config)
}

func getConfig() (*config.Config, error) {
	return config.GetConfigFrom(globalFlags.SystemConfigDir, globalFlags.LocalConfigDir)
}

func lockDir() string {
	return filepath.Join(globalFlags.Dir, "locks")
}
