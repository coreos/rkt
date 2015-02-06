// Copyright 2014 CoreOS, Inc.
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
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/coreos/rocket/pkg/keystore"
)

const (
	cliName        = "rkt"
	cliDescription = "rocket, the application container runner"

	defaultDataDir = "/var/lib/rkt"
)

var (
	globalFlagset = flag.NewFlagSet(cliName, flag.ExitOnError)
	tabOut        *tabwriter.Writer
	commands      []*Command // Commands should register themselves by appending
	globalFlags   = struct {
		Dir                string
		Debug              bool
		Help               bool
		InsecureSkipVerify bool
	}{}
)

func init() {
	globalFlagset.BoolVar(&globalFlags.Help, "help", false, "Print usage information and exit")
	globalFlagset.BoolVar(&globalFlags.Debug, "debug", false, "Print out more debug information to stderr")
	globalFlagset.StringVar(&globalFlags.Dir, "dir", defaultDataDir, "rocket data directory")
	globalFlagset.BoolVar(&globalFlags.InsecureSkipVerify, "insecure-skip-verify", false, "skip image or key verification")
}

type Command struct {
	Name        string       // Name of the Command and the string to use to invoke it
	Summary     string       // One-sentence summary of what the Command does
	Usage       string       // Usage options/arguments
	Description string       // Detailed description of command
	Flags       flag.FlagSet // Set of flags associated with this command

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

	// determine which Command should be run
	for _, c := range commands {
		if c.Name == args[0] {
			cmd = c
			if err := c.Flags.Parse(args[1:]); err != nil {
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

	if globalFlags.Debug {
		log.SetOutput(os.Stderr)
	}

	os.Exit(cmd.Run(cmd.Flags.Args()))
}

func maybeAddNewline(s string) string {
	if !strings.HasSuffix(s, "\n") {
		s = s + "\n"
	}
	return s
}

func stderr(format string, a ...interface{}) (int, error) {
	return fmt.Fprintf(os.Stderr, maybeAddNewline(format), a...)
}

func stdout(format string, a ...interface{}) (int, error) {
	return fmt.Fprintf(os.Stdout, maybeAddNewline(format), a...)
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

func containersDir() string {
	return filepath.Join(globalFlags.Dir, "containers")
}

func garbageDir() string {
	return filepath.Join(globalFlags.Dir, "garbage")
}

func getKeystore() *keystore.Keystore {
	if globalFlags.InsecureSkipVerify {
		return nil
	}
	return keystore.New(nil)
}
