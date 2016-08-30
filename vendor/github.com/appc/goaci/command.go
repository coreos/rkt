package main

// command provides an interface for named actions for command line
// purposes.
type command interface {
	// Name should return a name of a command usable at command
	// line.
	Name() string
	// Run should parse given args and perform some action. name
	// parameter is given for usage purposes.
	Run(name string, args []string) error
}
