package exql

import (
	"fmt"
	"strings"
)

// Order represents the order in which SQL results are sorted.
type Order uint8

// Possible values for Order
const (
	DefaultOrder = Order(iota)
	Ascendent
	Descendent
)

// SortColumn represents the column-order relation in an ORDER BY clause.
type SortColumn struct {
	Column Fragment
	Order
	hash hash
}

type sortColumnT struct {
	Column string
	Order  string
}

// SortColumns represents the columns in an ORDER BY clause.
type SortColumns struct {
	Columns []Fragment
	hash    hash
}

// OrderBy represents an ORDER BY clause.
type OrderBy struct {
	SortColumns Fragment
	hash        hash
}

type orderByT struct {
	SortColumns string
}

// JoinSortColumns creates and returns an array of column-order relations.
func JoinSortColumns(values ...Fragment) *SortColumns {
	return &SortColumns{Columns: values}
}

// JoinWithOrderBy creates an returns an OrderBy using the given SortColumns.
func JoinWithOrderBy(sc *SortColumns) *OrderBy {
	return &OrderBy{SortColumns: sc}
}

// Hash returns a unique identifier for the struct.
func (s *SortColumn) Hash() string {
	return s.hash.Hash(s)
}

// Compile transforms the SortColumn into an equivalent SQL representation.
func (s *SortColumn) Compile(layout *Template) (compiled string, err error) {

	if c, ok := layout.Read(s); ok {
		return c, nil
	}

	column, err := s.Column.Compile(layout)
	if err != nil {
		return "", err
	}

	orderBy, err := s.Order.Compile(layout)
	if err != nil {
		return "", err
	}

	data := sortColumnT{Column: column, Order: orderBy}

	compiled = mustParse(layout.SortByColumnLayout, data)

	layout.Write(s, compiled)

	return
}

// Hash returns a unique identifier for the struct.
func (s *SortColumns) Hash() string {
	return s.hash.Hash(s)
}

// Compile transforms the SortColumns into an equivalent SQL representation.
func (s *SortColumns) Compile(layout *Template) (compiled string, err error) {
	if z, ok := layout.Read(s); ok {
		return z, nil
	}

	z := make([]string, len(s.Columns))

	for i := range s.Columns {
		z[i], err = s.Columns[i].Compile(layout)
		if err != nil {
			return "", err
		}
	}

	compiled = strings.Join(z, layout.IdentifierSeparator)

	layout.Write(s, compiled)

	return
}

// Hash returns a unique identifier for the struct.
func (s *OrderBy) Hash() string {
	return s.hash.Hash(s)
}

// Compile transforms the SortColumn into an equivalent SQL representation.
func (s *OrderBy) Compile(layout *Template) (compiled string, err error) {
	if z, ok := layout.Read(s); ok {
		return z, nil
	}

	if s.SortColumns != nil {
		sortColumns, err := s.SortColumns.Compile(layout)
		if err != nil {
			return "", err
		}

		data := orderByT{
			SortColumns: sortColumns,
		}
		compiled = mustParse(layout.OrderByLayout, data)
	}

	layout.Write(s, compiled)

	return
}

// Hash returns a unique identifier.
func (s *Order) Hash() string {
	return fmt.Sprintf("%T.%d", s, uint8(*s))
}

// Compile transforms the SortColumn into an equivalent SQL representation.
func (s Order) Compile(layout *Template) (string, error) {
	switch s {
	case Ascendent:
		return layout.AscKeyword, nil
	case Descendent:
		return layout.DescKeyword, nil
	}
	return "", nil
}
