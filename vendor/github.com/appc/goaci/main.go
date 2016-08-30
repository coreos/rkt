package main

import (
	"fmt"
	"os"
	"sort"

	"github.com/appc/goaci/proj2aci"
)

type CmdLineError struct {
	what string
}

func (err *CmdLineError) Error() string {
	return fmt.Sprintf("Command line error: %s", err.what)
}

func newCmdLineError(format string, args ...interface{}) error {
	return &CmdLineError{
		what: fmt.Sprintf(format, args...),
	}
}

func main() {
	if err := mainWithError(); err != nil {
		proj2aci.Warn(err)
		if _, ok := err.(*CmdLineError); ok {
			printUsage()
		}
		os.Exit(1)
	}
}

func mainWithError() error {
	proj2aci.InitDebug()
	if len(os.Args) < 2 {
		return newCmdLineError("No command specified")
	}
	if c, ok := commandsMap[os.Args[1]]; ok {
		name := fmt.Sprintf("%s %s", os.Args[0], os.Args[1])
		return c.Run(name, os.Args[2:])
	} else {
		return newCmdLineError("No such command: %q", os.Args[1])
	}
}

func printUsage() {
	fmt.Println("Available commands:")
	commands := make([]string, 0, len(commandsMap))
	for c := range commandsMap {
		commands = append(commands, c)
	}
	sort.Strings(commands)
	for _, c := range commands {
		fmt.Printf("  %s\n", c)
	}
	fmt.Printf("Type %s <command> --help to get possible options for chosen command\n", os.Args[0])
}
