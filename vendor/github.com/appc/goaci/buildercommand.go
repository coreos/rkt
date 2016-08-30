package main

import (
	"flag"
	"fmt"

	"github.com/appc/goaci/proj2aci"
)

// parameterMapper is an interface which should handle command line
// parameter handling specific to a proj2aci.BuilderCustomizations
// implementation.
type parameterMapper interface {
	Name() string
	SetupParameters(parameters *flag.FlagSet)
	GetBuilderCustomizations() proj2aci.BuilderCustomizations
}

// builderCommand is an implementation of command interface which
// mainly maps command line parameters to proj2aci.Builder's
// configuration and runs the builder.
type builderCommand struct {
	mapper parameterMapper
}

func newBuilderCommand(mapper parameterMapper) command {
	return &builderCommand{
		mapper: mapper,
	}
}

func (cmd *builderCommand) Name() string {
	custom := cmd.mapper.GetBuilderCustomizations()
	return custom.Name()
}

func (cmd *builderCommand) Run(name string, args []string) error {
	parameters := flag.NewFlagSet(name, flag.ExitOnError)
	cmd.mapper.SetupParameters(parameters)
	if err := parameters.Parse(args); err != nil {
		return err
	}
	if len(parameters.Args()) != 1 {
		return fmt.Errorf("Expected exactly one project to build, got %d", len(args))
	}
	custom := cmd.mapper.GetBuilderCustomizations()
	custom.GetCommonConfiguration().Project = parameters.Args()[0]
	builder := proj2aci.NewBuilder(custom)
	return builder.Run()
}
