package exql

// Returning represents a RETURNING clause.
type Returning struct {
	*Columns
	hash hash
}

// Hash returns a unique identifier for the struct.
func (r *Returning) Hash() string {
	return r.hash.Hash(r)
}

// ReturningColumns creates and returns an array of Column.
func ReturningColumns(columns ...Fragment) *Returning {
	return &Returning{Columns: &Columns{Columns: columns}}
}

// Compile transforms the clause into its equivalent SQL representation.
func (r *Returning) Compile(layout *Template) (compiled string, err error) {
	if z, ok := layout.Read(r); ok {
		return z, nil
	}

	compiled, err = r.Columns.Compile(layout)
	if err != nil {
		return "", err
	}

	layout.Write(r, compiled)

	return
}
