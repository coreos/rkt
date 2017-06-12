package exql

import (
	//"database/sql/driver"
	"fmt"
	//"log"
	"strings"
)

// ValueGroups represents an array of value groups.
type ValueGroups struct {
	Values []*Values
	hash   hash
}

// Values represents an array of Value.
type Values struct {
	Values []Fragment
	hash   hash
}

// Value represents an escaped SQL value.
type Value struct {
	V    interface{}
	hash hash
}

// NewValue creates and returns a Value.
func NewValue(v interface{}) *Value {
	return &Value{V: v}
}

// NewValueGroup creates and returns an array of values.
func NewValueGroup(v ...Fragment) *Values {
	return &Values{Values: v}
}

// Hash returns a unique identifier for the struct.
func (v *Value) Hash() string {
	return v.hash.Hash(v)
}

// Compile transforms the Value into an equivalent SQL representation.
func (v *Value) Compile(layout *Template) (compiled string, err error) {

	if z, ok := layout.Read(v); ok {
		return z, nil
	}

	switch t := v.V.(type) {
	case Raw:
		compiled, err = t.Compile(layout)
		if err != nil {
			return "", err
		}
	case Fragment:
		compiled, err = t.Compile(layout)
		if err != nil {
			return "", err
		}
	default:
		compiled = mustParse(layout.ValueQuote, RawValue(fmt.Sprintf(`%v`, v.V)))
	}

	layout.Write(v, compiled)

	return
}

/*
func (v *Value) Scan(src interface{}) error {
	log.Println("Scan(", src, ") on", v.V)
	return nil
}

func (v *Value) Value() (driver.Value, error) {
	log.Println("Value() on", v.V)
	return v.V, nil
}
*/

// Hash returns a unique identifier for the struct.
func (vs *Values) Hash() string {
	return vs.hash.Hash(vs)
}

// Compile transforms the Values into an equivalent SQL representation.
func (vs *Values) Compile(layout *Template) (compiled string, err error) {
	if c, ok := layout.Read(vs); ok {
		return c, nil
	}

	l := len(vs.Values)
	if l > 0 {
		chunks := make([]string, 0, l)
		for i := 0; i < l; i++ {
			chunk, err := vs.Values[i].Compile(layout)
			if err != nil {
				return "", err
			}
			chunks = append(chunks, chunk)
		}
		compiled = mustParse(layout.ClauseGroup, strings.Join(chunks, layout.ValueSeparator))
	}
	layout.Write(vs, compiled)
	return
}

/*
func (vs Values) Scan(src interface{}) error {
	log.Println("Values.Scan(", src, ")")
	return nil
}

func (vs Values) Value() (driver.Value, error) {
	log.Println("Values.Value()")
	return vs, nil
}
*/

// Hash returns a unique identifier for the struct.
func (vg *ValueGroups) Hash() string {
	return vg.hash.Hash(vg)
}

// Compile transforms the ValueGroups into an equivalent SQL representation.
func (vg *ValueGroups) Compile(layout *Template) (compiled string, err error) {
	if c, ok := layout.Read(vg); ok {
		return c, nil
	}

	l := len(vg.Values)
	if l > 0 {
		chunks := make([]string, 0, l)
		for i := 0; i < l; i++ {
			chunk, err := vg.Values[i].Compile(layout)
			if err != nil {
				return "", err
			}
			chunks = append(chunks, chunk)
		}
		compiled = strings.Join(chunks, layout.ValueSeparator)
	}

	layout.Write(vg, compiled)
	return
}

// JoinValueGroups creates a new *ValueGroups object.
func JoinValueGroups(values ...*Values) *ValueGroups {
	return &ValueGroups{Values: values}
}
