package main

var (
	commandsMap map[string]command = make(map[string]command)
)

func init() {
	commands := []command{
		newBuilderCommand(newGoParameterMapper()),
		newBuilderCommand(newCmakeParameterMapper()),
	}
	for _, c := range commands {
		commandsMap[c.Name()] = c
	}
}
