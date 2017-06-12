package exql

import (
	"strings"
)

type innerJoinT struct {
	Type  string
	Table string
	On    string
	Using string
}

// Joins represents the union of different join conditions.
type Joins struct {
	Conditions []Fragment
	hash       hash
}

// Hash returns a unique identifier for the struct.
func (j *Joins) Hash() string {
	return j.hash.Hash(j)
}

// Compile transforms the Where into an equivalent SQL representation.
func (j *Joins) Compile(layout *Template) (compiled string, err error) {
	if c, ok := layout.Read(j); ok {
		return c, nil
	}

	l := len(j.Conditions)

	chunks := make([]string, 0, l)

	if l > 0 {
		for i := 0; i < l; i++ {
			chunk, err := j.Conditions[i].Compile(layout)
			if err != nil {
				return "", err
			}
			chunks = append(chunks, chunk)
		}
	}

	compiled = strings.Join(chunks, " ")

	layout.Write(j, compiled)

	return
}

// JoinConditions creates a Joins object.
func JoinConditions(joins ...*Join) *Joins {
	fragments := make([]Fragment, len(joins))
	for i := range fragments {
		fragments[i] = joins[i]
	}
	return &Joins{Conditions: fragments}
}

// Join represents a generic JOIN statement.
type Join struct {
	Type string
	*Table
	*On
	*Using
	hash hash
}

// Hash returns a unique identifier for the struct.
func (j *Join) Hash() string {
	return j.hash.Hash(j)
}

// Compile transforms the Join into its equivalent SQL representation.
func (j *Join) Compile(layout *Template) (compiled string, err error) {
	if c, ok := layout.Read(j); ok {
		return c, nil
	}

	if j.Table == nil {
		return "", nil
	}

	table, err := j.Table.Compile(layout)
	if err != nil {
		return "", err
	}

	on, err := layout.doCompile(j.On)
	if err != nil {
		return "", err
	}

	using, err := layout.doCompile(j.Using)
	if err != nil {
		return "", err
	}

	data := innerJoinT{
		Type:  j.Type,
		Table: table,
		On:    on,
		Using: using,
	}

	compiled = mustParse(layout.JoinLayout, data)
	layout.Write(j, compiled)
	return
}

// On represents JOIN conditions.
type On Where

// Hash returns a unique identifier.
func (o *On) Hash() string {
	return o.hash.Hash(o)
}

// Compile transforms the On into an equivalent SQL representation.
func (o *On) Compile(layout *Template) (compiled string, err error) {
	if c, ok := layout.Read(o); ok {
		return c, nil
	}

	grouped, err := groupCondition(layout, o.Conditions, mustParse(layout.ClauseOperator, layout.AndKeyword))
	if err != nil {
		return "", err
	}

	if grouped != "" {
		compiled = mustParse(layout.OnLayout, conds{grouped})
	}

	layout.Write(o, compiled)
	return
}

// Using represents a USING function.
type Using Columns

type usingT struct {
	Columns string
}

// Hash returns a unique identifier.
func (u *Using) Hash() string {
	return u.hash.Hash(u)
}

// Compile transforms the Using into an equivalent SQL representation.
func (u *Using) Compile(layout *Template) (compiled string, err error) {
	if u == nil {
		return "", nil
	}

	if c, ok := layout.Read(u); ok {
		return c, nil
	}

	if len(u.Columns) > 0 {
		c := Columns(*u)
		columns, err := c.Compile(layout)
		if err != nil {
			return "", err
		}
		data := usingT{Columns: columns}
		compiled = mustParse(layout.UsingLayout, data)
	}

	layout.Write(u, compiled)
	return
}
