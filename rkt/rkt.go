package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"
)

const (
	cliName        = "rkt"
	cliDescription = "rocket, the application container runner"

	defaultDataDir = "/var/lib/rkt"
)

var (
	globalFlagset = flag.NewFlagSet(cliName, flag.ExitOnError)
	out           *tabwriter.Writer
	commands      []*Command // Commands should register themselves by appending
	globalFlags   = struct {
		Dir   string
		Debug bool
		Help  bool
	}{}
)

func init() {
	globalFlagset.BoolVar(&globalFlags.Help, "help", false, "Print usage information and exit")
	globalFlagset.BoolVar(&globalFlags.Debug, "debug", false, "Print out more debug information to stderr")
	globalFlagset.StringVar(&globalFlags.Dir, "dir", defaultDataDir, "rocket data directory")
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
	out = new(tabwriter.Writer)
	out.Init(os.Stdout, 0, 8, 1, '\t', 0)
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
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(2)
			}
			break
		}
	}

	if cmd == nil {
		fmt.Fprintf(os.Stderr, "%v: unknown subcommand: %q\n", cliName, args[0])
		fmt.Fprintf(os.Stderr, "Run '%v help' for usage.\n", cliName)
		os.Exit(2)
	}
	os.Exit(cmd.Run(cmd.Flags.Args()))
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

func containersDir(rktDir string) string {
	return filepath.Join(rktDir, "containers")
}
