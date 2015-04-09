package main

import (
	"flag"
	"fmt"
	//"io/ioutil"
	"os"
	//"strings"
)

var (
	globalFlagset = flag.NewFlagSet("inspect", flag.ExitOnError)
	globalFlags   = struct {
		PrintMsg string
		PrintEnv string
		CheckCwd string
		ExitCode int
	}{}
)

func init() {
	globalFlagset.StringVar(&globalFlags.PrintMsg, "print-msg", "", "Print the message given as parameter")
	globalFlagset.StringVar(&globalFlags.CheckCwd, "check-cwd", "", "Check if the current working directory is the one specified")
	globalFlagset.StringVar(&globalFlags.PrintEnv, "print-env", "", "Print the specified environment variable")
	globalFlagset.IntVar(&globalFlags.ExitCode, "exit-code", 0, "Return this exit code")
}

func main() {
	globalFlagset.Parse(os.Args[1:])
	args := globalFlagset.Args()
	if len(args) > 0 {
		fmt.Fprintln(os.Stderr, "Wrong parameters\n")
		os.Exit(1)
	}

	if globalFlags.PrintMsg != "" {
		fmt.Fprintf(os.Stdout, "%s\n", globalFlags.PrintMsg)
	}

	if globalFlags.PrintEnv != "" {
		fmt.Fprintf(os.Stdout, "%s=%s\n", globalFlags.PrintEnv, os.Getenv(globalFlags.PrintEnv))
	}

	if globalFlags.CheckCwd != "" {
		wd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Cannot get working directory: %v\n", err)
			os.Exit(1)
		}
		if wd != globalFlags.CheckCwd {
			fmt.Fprintf(os.Stderr, "Working directory: %q. Expected: %q.\n", wd, globalFlags.CheckCwd)
			os.Exit(1)
		}
	}

	os.Exit(globalFlags.ExitCode)
}
