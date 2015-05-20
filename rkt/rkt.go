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
	"flag"
	"fmt"
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
	globalFlagset = flag.NewFlagSet(cliName, flag.ExitOnError)
	tabOut        *tabwriter.Writer
	commands      []*Command // Commands should register themselves by appending
	subCommands   = make(map[string][]*Command)
	globalFlags   = struct {
		Dir                string
		SystemConfigDir    string
		LocalConfigDir     string
		Debug              bool
		Help               bool
		InsecureSkipVerify bool
	}{}
)

func init() {
	globalFlagset.BoolVar(&globalFlags.Help, "help", false, "Print usage information and exit")
	globalFlagset.BoolVar(&globalFlags.Debug, "debug", false, "Print out more debug information to stderr")
	globalFlagset.StringVar(&globalFlags.Dir, "dir", defaultDataDir, "rkt data directory")
	globalFlagset.StringVar(&globalFlags.SystemConfigDir, "system-config", common.DefaultSystemConfigDir, "system configuration directory")
	globalFlagset.StringVar(&globalFlags.LocalConfigDir, "local-config", common.DefaultLocalConfigDir, "local configuration directory")
	globalFlagset.BoolVar(&globalFlags.InsecureSkipVerify, "insecure-skip-verify", false, "skip all TLS, image or fingerprint verification")
}

type Command struct {
	Name                 string        // Name of the Command and the string to use to invoke it
	Summary              string        // One-sentence summary of what the Command does
	Usage                string        // Usage options/arguments
	Description          string        // Detailed description of command
	Flags                *flag.FlagSet // Set of flags associated with this command
	WantsFlagsTerminator bool          // Include the potential "--" flags terminator in args for Run

	Run func(args []string) int // Run a command with the given arguments, return exit status

}

func init() {
	tabOut = new(tabwriter.Writer)
	tabOut.Init(os.Stdout, 0, 8, 1, '\t', 0)
}

func main() {
	// parse global arguments
	globalFlagset.Parse(os.Args[1:])
	args := globalFlagset.Args()
	if len(args) < 1 || globalFlags.Help {
		args = []string{"help"}
	}

	var cmd *Command
	subArgs := args[1:]

	// determine which Command should be run
	for _, c := range commands {
		if c.Name == args[0] {
			cmd = c
			if err := c.Flags.Parse(subArgs); err != nil {
				stderr("%v", err)
				os.Exit(2)
			}
			break
		}
	}

	if cmd == nil {
		stderr("%v: unknown subcommand: %q", cliName, args[0])
		stderr("Run '%v help' for usage.", cliName)
		os.Exit(2)
	}

	if !globalFlags.Debug {
		log.SetOutput(ioutil.Discard)
	}

	// XXX(vc): Flags.Args() stops parsing at "--" but swallows it in doing so.
	// This interferes with parseApps() so we detect that here and reclaim "--",
	// passing it to the subcommand with the rest of the arguments.
	subArgsConsumed := len(subArgs) - cmd.Flags.NArg()
	if cmd.WantsFlagsTerminator && subArgsConsumed > 0 && subArgs[subArgsConsumed-1] == "--" {
		subArgsConsumed--
	}
	os.Exit(cmd.Run(subArgs[subArgsConsumed:]))
}

func stderr(format string, a ...interface{}) {
	out := fmt.Sprintf(format, a...)
	fmt.Fprintln(os.Stderr, strings.TrimSuffix(out, "\n"))
}

func stdout(format string, a ...interface{}) {
	out := fmt.Sprintf(format, a...)
	fmt.Fprintln(os.Stdout, strings.TrimSuffix(out, "\n"))
}

func getAllFlags() (flags []*flag.Flag) {
	return getFlags(globalFlagset)
}

func getFlags(flagset *flag.FlagSet) (flags []*flag.Flag) {
	flags = make([]*flag.Flag, 0)
	flagset.VisitAll(func(f *flag.Flag) {
		flags = append(flags, f)
	})
	return
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
	if globalFlags.InsecureSkipVerify {
		return nil
	}
	config := keystore.NewConfig(globalFlags.SystemConfigDir, globalFlags.LocalConfigDir)
	return keystore.New(config)
}

func getConfig() (*config.Config, error) {
	return config.GetConfigFrom(globalFlags.SystemConfigDir, globalFlags.LocalConfigDir)
}
