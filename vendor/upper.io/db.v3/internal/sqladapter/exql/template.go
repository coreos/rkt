package exql

import (
	"bytes"
	"sync"
	"text/template"

	"upper.io/db.v3/internal/cache"
)

// Type is the type of SQL query the statement represents.
type Type uint

// Values for Type.
const (
	NoOp = Type(iota)

	Truncate
	DropTable
	DropDatabase
	Count
	Insert
	Select
	Update
	Delete

	SQL
)

type (
	// Limit represents the SQL limit in a query.
	Limit int
	// Offset represents the SQL offset in a query.
	Offset int
)

var (
	templateCache = templateMap{M: make(map[string]*template.Template)}
)

// Template is an SQL template.
type Template struct {
	AndKeyword          string
	AscKeyword          string
	AssignmentOperator  string
	ClauseGroup         string
	ClauseOperator      string
	ColumnAliasLayout   string
	ColumnSeparator     string
	ColumnValue         string
	CountLayout         string
	DefaultOperator     string
	DeleteLayout        string
	DescKeyword         string
	DropDatabaseLayout  string
	DropTableLayout     string
	GroupByLayout       string
	IdentifierQuote     string
	IdentifierSeparator string
	InsertLayout        string
	JoinLayout          string
	NotKeyword          string
	OnLayout            string
	OrKeyword           string
	OrderByLayout       string
	SelectLayout        string
	SortByColumnLayout  string
	TableAliasLayout    string
	TruncateLayout      string
	UpdateLayout        string
	UsingLayout         string
	ValueQuote          string
	ValueSeparator      string
	WhereLayout         string

	*cache.Cache
}

func mustParse(text string, data interface{}) string {
	var b bytes.Buffer
	var ok bool

	v, ok := templateCache.Get(text)
	if !ok {
		v = template.Must(template.New("").Parse(text))
		templateCache.Set(text, v)
	}

	if err := v.Execute(&b, data); err != nil {
		panic("There was an error compiling the following template:\n" + text + "\nError was: " + err.Error())
	}

	return b.String()
}

type templateMap struct {
	sync.RWMutex
	M map[string]*template.Template
}

func (m *templateMap) Get(k string) (*template.Template, bool) {
	m.RLock()
	defer m.RUnlock()
	v, ok := m.M[k]
	return v, ok
}

func (m *templateMap) Set(k string, v *template.Template) {
	m.Lock()
	defer m.Unlock()
	m.M[k] = v
}
