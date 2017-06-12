package exql

// Database represents a SQL database.
type Database struct {
	Name string
	hash hash
}

// DatabaseWithName returns a Database with the given name.
func DatabaseWithName(name string) *Database {
	return &Database{Name: name}
}

// Hash returns a unique identifier for the struct.
func (d *Database) Hash() string {
	return d.hash.Hash(d)
}

// Compile transforms the Database into an equivalent SQL representation.
func (d *Database) Compile(layout *Template) (compiled string, err error) {
	if c, ok := layout.Read(d); ok {
		return c, nil
	}

	compiled = mustParse(layout.IdentifierQuote, Raw{Value: d.Name})

	layout.Write(d, compiled)
	return
}
