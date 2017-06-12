package exql

import (
	"fmt"
	"strings"
)

type columnT struct {
	Name  string
	Alias string
}

// Column represents a SQL column.
type Column struct {
	Name  interface{}
	Alias string
	hash  hash
}

// ColumnWithName creates and returns a Column with the given name.
func ColumnWithName(name string) *Column {
	return &Column{Name: name}
}

// Hash returns a unique identifier for the struct.
func (c *Column) Hash() string {
	return c.hash.Hash(c)
}

// Compile transforms the ColumnValue into an equivalent SQL representation.
func (c *Column) Compile(layout *Template) (compiled string, err error) {
	if z, ok := layout.Read(c); ok {
		return z, nil
	}

	alias := c.Alias

	switch value := c.Name.(type) {
	case string:
		input := trimString(value)

		chunks := separateByAS(input)

		if len(chunks) == 1 {
			chunks = separateBySpace(input)
		}

		name := chunks[0]

		nameChunks := strings.SplitN(name, layout.ColumnSeparator, 2)

		for i := range nameChunks {
			nameChunks[i] = trimString(nameChunks[i])
			if nameChunks[i] == "*" {
				continue
			}
			nameChunks[i] = mustParse(layout.IdentifierQuote, Raw{Value: nameChunks[i]})
		}

		compiled = strings.Join(nameChunks, layout.ColumnSeparator)

		if len(chunks) > 1 {
			alias = trimString(chunks[1])
			alias = mustParse(layout.IdentifierQuote, Raw{Value: alias})
		}
	case Raw:
		compiled = value.String()
	default:
		compiled = fmt.Sprintf("%v", c.Name)
	}

	if alias != "" {
		compiled = mustParse(layout.ColumnAliasLayout, columnT{compiled, alias})
	}

	layout.Write(c, compiled)

	return
}
