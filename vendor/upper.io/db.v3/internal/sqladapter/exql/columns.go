package exql

import (
	"strings"
)

// Columns represents an array of Column.
type Columns struct {
	Columns []Fragment
	hash    hash
}

// Hash returns a unique identifier.
func (c *Columns) Hash() string {
	return c.hash.Hash(c)
}

// JoinColumns creates and returns an array of Column.
func JoinColumns(columns ...Fragment) *Columns {
	return &Columns{Columns: columns}
}

// OnConditions creates and retuens a new On.
func OnConditions(conditions ...Fragment) *On {
	return &On{Conditions: conditions}
}

// UsingColumns builds a Using from the given columns.
func UsingColumns(columns ...Fragment) *Using {
	return &Using{Columns: columns}
}

func (c *Columns) Append(a *Columns) *Columns {
	c.Columns = append(c.Columns, a.Columns...)
	return c
}

// Compile transforms the Columns into an equivalent SQL representation.
func (c *Columns) Compile(layout *Template) (compiled string, err error) {

	if z, ok := layout.Read(c); ok {
		return z, nil
	}

	l := len(c.Columns)

	if l > 0 {
		out := make([]string, l)

		for i := 0; i < l; i++ {
			out[i], err = c.Columns[i].Compile(layout)
			if err != nil {
				return "", err
			}
		}

		compiled = strings.Join(out, layout.IdentifierSeparator)
	}

	layout.Write(c, compiled)

	return
}
