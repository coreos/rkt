package exql

import (
	"errors"
	"reflect"
	"strings"

	"upper.io/db.v3/internal/cache"
)

var errUnknownTemplateType = errors.New("Unknown template type")

// Statement represents different kinds of SQL statements.
type Statement struct {
	Type
	Table        Fragment
	Database     Fragment
	Columns      Fragment
	Values       Fragment
	Distinct     bool
	ColumnValues Fragment
	OrderBy      Fragment
	GroupBy      Fragment
	Joins        Fragment
	Where        Fragment
	Returning    Fragment

	Limit
	Offset

	SQL string

	hash    hash
	amendFn func(string) string
}

type statementT struct {
	Table        string
	Database     string
	Columns      string
	Values       string
	Distinct     bool
	ColumnValues string
	OrderBy      string
	GroupBy      string
	Where        string
	Joins        string
	Returning    string
	Limit
	Offset
}

func (layout *Template) doCompile(c Fragment) (string, error) {
	if c != nil && !reflect.ValueOf(c).IsNil() {
		return c.Compile(layout)
	}
	return "", nil
}

func getHash(h cache.Hashable) string {
	if h != nil && !reflect.ValueOf(h).IsNil() {
		return h.Hash()
	}
	return ""
}

// Hash returns a unique identifier for the struct.
func (s *Statement) Hash() string {
	return s.hash.Hash(s)
}

func (s *Statement) SetAmendment(amendFn func(string) string) {
	s.amendFn = amendFn
}

func (s *Statement) Amend(in string) string {
	if s.amendFn == nil {
		return in
	}
	return s.amendFn(in)
}

// Compile transforms the Statement into an equivalent SQL query.
func (s *Statement) Compile(layout *Template) (compiled string, err error) {
	if s.Type == SQL {
		// No need to hit the cache.
		return s.SQL, nil
	}

	if z, ok := layout.Read(s); ok {
		return s.Amend(z), nil
	}

	data := statementT{
		Limit:    s.Limit,
		Offset:   s.Offset,
		Distinct: s.Distinct,
	}

	data.Table, err = layout.doCompile(s.Table)
	if err != nil {
		return "", err
	}

	data.Database, err = layout.doCompile(s.Database)
	if err != nil {
		return "", err
	}

	data.Columns, err = layout.doCompile(s.Columns)
	if err != nil {
		return "", err
	}

	data.Values, err = layout.doCompile(s.Values)
	if err != nil {
		return "", err
	}

	data.ColumnValues, err = layout.doCompile(s.ColumnValues)
	if err != nil {
		return "", err
	}

	data.OrderBy, err = layout.doCompile(s.OrderBy)
	if err != nil {
		return "", err
	}

	data.GroupBy, err = layout.doCompile(s.GroupBy)
	if err != nil {
		return "", err
	}

	data.Where, err = layout.doCompile(s.Where)
	if err != nil {
		return "", err
	}

	data.Returning, err = layout.doCompile(s.Returning)
	if err != nil {
		return "", err
	}

	data.Joins, err = layout.doCompile(s.Joins)
	if err != nil {
		return "", err
	}

	switch s.Type {
	case Truncate:
		compiled = mustParse(layout.TruncateLayout, data)
	case DropTable:
		compiled = mustParse(layout.DropTableLayout, data)
	case DropDatabase:
		compiled = mustParse(layout.DropDatabaseLayout, data)
	case Count:
		compiled = mustParse(layout.CountLayout, data)
	case Select:
		compiled = mustParse(layout.SelectLayout, data)
	case Delete:
		compiled = mustParse(layout.DeleteLayout, data)
	case Update:
		compiled = mustParse(layout.UpdateLayout, data)
	case Insert:
		compiled = mustParse(layout.InsertLayout, data)
	default:
		return "", errUnknownTemplateType
	}

	compiled = strings.TrimSpace(compiled)
	layout.Write(s, compiled)

	return s.Amend(compiled), nil
}

// RawSQL represents a raw SQL statement.
func RawSQL(s string) *Statement {
	return &Statement{
		Type: SQL,
		SQL:  s,
	}
}
