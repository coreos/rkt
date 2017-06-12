package exql

import (
	"fmt"
)

var (
	_ = fmt.Stringer(&Raw{})
)

// Raw represents a value that is meant to be used in a query without escaping.
type Raw struct {
	Value string // Value should not be modified after assigned.
	hash  hash
}

// RawValue creates and returns a new raw value.
func RawValue(v string) *Raw {
	return &Raw{Value: v}
}

// Hash returns a unique identifier for the struct.
func (r *Raw) Hash() string {
	return r.hash.Hash(r)
}

// Compile returns the raw value.
func (r *Raw) Compile(*Template) (string, error) {
	return r.Value, nil
}

// String returns the raw value.
func (r *Raw) String() string {
	return r.Value
}
