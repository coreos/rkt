package exql

import (
	"strings"
)

type tableT struct {
	Name  string
	Alias string
}

// Table struct represents a SQL table.
type Table struct {
	Name interface{}
	hash hash
}

func quotedTableName(layout *Template, input string) string {
	input = trimString(input)

	// chunks := reAliasSeparator.Split(input, 2)
	chunks := separateByAS(input)

	if len(chunks) == 1 {
		// chunks = reSpaceSeparator.Split(input, 2)
		chunks = separateBySpace(input)
	}

	name := chunks[0]

	nameChunks := strings.SplitN(name, layout.ColumnSeparator, 2)

	for i := range nameChunks {
		// nameChunks[i] = strings.TrimSpace(nameChunks[i])
		nameChunks[i] = trimString(nameChunks[i])
		nameChunks[i] = mustParse(layout.IdentifierQuote, Raw{Value: nameChunks[i]})
	}

	name = strings.Join(nameChunks, layout.ColumnSeparator)

	var alias string

	if len(chunks) > 1 {
		// alias = strings.TrimSpace(chunks[1])
		alias = trimString(chunks[1])
		alias = mustParse(layout.IdentifierQuote, Raw{Value: alias})
	}

	return mustParse(layout.TableAliasLayout, tableT{name, alias})
}

// TableWithName creates an returns a Table with the given name.
func TableWithName(name string) *Table {
	return &Table{Name: name}
}

// Hash returns a string hash of the table value.
func (t *Table) Hash() string {
	return t.hash.Hash(t)
}

// Compile transforms a table struct into a SQL chunk.
func (t *Table) Compile(layout *Template) (compiled string, err error) {

	if z, ok := layout.Read(t); ok {
		return z, nil
	}

	switch value := t.Name.(type) {
	case string:
		if t.Name == "" {
			return
		}

		// Splitting tables by a comma
		parts := separateByComma(value)

		l := len(parts)

		for i := 0; i < l; i++ {
			parts[i] = quotedTableName(layout, parts[i])
		}

		compiled = strings.Join(parts, layout.IdentifierSeparator)
	case Raw:
		compiled = value.String()
	}

	layout.Write(t, compiled)

	return
}
