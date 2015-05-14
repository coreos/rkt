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
	"strings"
	"text/template"

	"github.com/coreos/rkt/version"
)

var (
	cmdHelp = &Command{
		Name:        "help",
		Summary:     "Show a list of commands or help for one command",
		Usage:       "[COMMAND]",
		Description: "Show a list of commands or detailed help for one command",
		Run:         runHelp,
		Flags:       &helpFlags,
	}
	helpFlags flag.FlagSet

	globalUsageTemplate     *template.Template
	commandUsageTemplate    *template.Template
	subCommandUsageTemplate *template.Template
	templFuncs              = template.FuncMap{
		"descToLines": func(s string) []string {
			// trim leading/trailing whitespace and split into slice of lines
			return strings.Split(strings.Trim(s, "\n\t "), "\n")
		},
		"printOption": func(name, defvalue, usage string) string {
			prefix := "--"
			if len(name) == 1 {
				prefix = "-"
			}
			return fmt.Sprintf("\t%s%s=%s\t%s", prefix, name, defvalue, usage)
		},
	}
)

func init() {
	commands = append(commands, cmdHelp)

	globalUsageTemplate = template.Must(template.New("global_usage").Funcs(templFuncs).Parse(`
NAME:
{{printf "\t%s - %s" .Executable .Description}}

USAGE: 
{{printf "\t%s" .Executable}} [global options] <command> [command options] [arguments...]

VERSION:
{{printf "\t%s" .Version}}

COMMANDS:{{range .Commands}}
{{printf "\t%s\t%s" .Name .Summary}}{{end}}

GLOBAL OPTIONS:{{range .Flags}}
{{printOption .Name .DefValue .Usage}}{{end}}

Run "{{.Executable}} help <command>" for more details on a specific command.
`[1:]))
	commandUsageTemplate = template.Must(template.New("command_usage").Funcs(templFuncs).Parse(`
NAME:
{{printf "\t%s - %s" .Cmd.Name .Cmd.Summary}}

USAGE:
{{printf "\t%s %s %s" .Executable .Cmd.Name .Cmd.Usage}}

DESCRIPTION:
{{range $line := descToLines .Cmd.Description}}{{printf "\t%s" $line}}
{{end}}
{{if .CmdFlags}}OPTIONS:{{range .CmdFlags}}
{{printOption .Name .DefValue .Usage}}{{end}}

{{end}}For help on global options run "{{.Executable}} help"
`[1:]))
	subCommandUsageTemplate = template.Must(template.New("subcommand_usage").Funcs(templFuncs).Parse(`
NAME:
{{printf "\t%s %s - %s" .CmdName .SubCmd.Name .SubCmd.Summary}}

USAGE:
{{printf "\t%s %s %s %s" .Executable .CmdName .SubCmd.Name .SubCmd.Usage}}

DESCRIPTION:
{{range $line := descToLines .SubCmd.Description}}{{printf "\t%s" $line}}
{{end}}
{{if .SubCmdFlags}}OPTIONS:{{range .SubCmdFlags}}
{{printOption .Name .DefValue .Usage}}{{end}}
{{end}}
`[1:]))
}

func runHelp(args []string) (exit int) {
	if len(args) < 1 {
		printGlobalUsage()
		return
	}

	if len(args) == 1 {
		if err := printCommandUsageByName(args[0]); err != nil {
			printGlobalUsage()
			stderr("\nHelp error: %v\n", err)
			return 1
		}
		return
	}

	// Help for sub-commands.
	if err := printSubCommandUsageByName(args[0], args[1], subCommands[args[0]]); err != nil {
		printGlobalUsage()
		stderr("\nHelp error: %v\n", err)
		return 1
	}
	return
}

func printGlobalUsage() {
	globalUsageTemplate.Execute(tabOut, struct {
		Executable  string
		Commands    []*Command
		Flags       []*flag.Flag
		Description string
		Version     string
	}{
		cliName,
		commands,
		getAllFlags(),
		cliDescription,
		version.Version,
	})
	tabOut.Flush()
}

func printCommandUsage(cmd *Command) {
	commandUsageTemplate.Execute(tabOut, struct {
		Executable string
		Cmd        *Command
		CmdFlags   []*flag.Flag
	}{
		cliName,
		cmd,
		getFlags(cmd.Flags),
	})
	tabOut.Flush()
}

func printCommandUsageByName(name string) error {
	var cmd *Command

	for _, c := range commands {
		if c.Name == name {
			cmd = c
			break
		}
	}

	if cmd == nil {
		return fmt.Errorf("unrecognized command: %s", name)
	}

	printCommandUsage(cmd)

	return nil
}

func printSubCommandUsage(cmdName string, subCmd *Command) {
	subCommandUsageTemplate.Execute(tabOut, struct {
		Executable  string
		CmdName     string
		SubCmd      *Command
		SubCmdFlags []*flag.Flag
	}{
		cliName,
		cmdName,
		subCmd,
		getFlags(subCmd.Flags),
	})
	tabOut.Flush()
}

func printSubCommandUsageByName(name, subName string, subCommands []*Command) error {
	var cmd *Command

	for _, c := range subCommands {
		if c.Name == subName {
			cmd = c
			break
		}
	}

	if cmd == nil {
		return fmt.Errorf("unrecognized sub-command: %s", subName)
	}

	printSubCommandUsage(name, cmd)

	return nil
}
