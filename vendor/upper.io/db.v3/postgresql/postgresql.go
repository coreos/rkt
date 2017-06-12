// Copyright (c) 2012-present The upper.io/db authors. All rights reserved.
//
// Permission is hereby granted, free of charge, to any person obtaining
// a copy of this software and associated documentation files (the
// "Software"), to deal in the Software without restriction, including
// without limitation the rights to use, copy, modify, merge, publish,
// distribute, sublicense, and/or sell copies of the Software, and to
// permit persons to whom the Software is furnished to do so, subject to
// the following conditions:
//
// The above copyright notice and this permission notice shall be
// included in all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
// EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
// MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND
// NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE
// LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION
// OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION
// WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

package postgresql // import "upper.io/db.v3/postgresql"

import (
	"database/sql"

	"upper.io/db.v3"

	"upper.io/db.v3/internal/sqladapter"
	"upper.io/db.v3/lib/sqlbuilder"
)

const sqlDriver = `postgres`

// Adapter is the unique name that you can use to refer to this adapter.
const Adapter = `postgresql`

func init() {
	sqlbuilder.RegisterAdapter(Adapter, &sqlbuilder.AdapterFuncMap{
		New:   New,
		NewTx: NewTx,
		Open:  Open,
	})
}

// Open opens a new connection with the PostgreSQL server. The returned session
// is validated first by Ping and then with a test query before being returned.
// You may call Open() just once and use it on multiple goroutines on a
// long-running program. See https://golang.org/pkg/database/sql/#Open and
// http://go-database-sql.org/accessing.html
func Open(settings db.ConnectionURL) (sqlbuilder.Database, error) {
	d := newDatabase(settings)
	if err := d.Open(settings); err != nil {
		return nil, err
	}
	return d, nil
}

// NewTx wraps a regular *sql.Tx transaction and returns a new upper-db
// transaction backed by it.
func NewTx(sqlTx *sql.Tx) (sqlbuilder.Tx, error) {
	d := newDatabase(nil)

	// Binding with sqladapter's logic.
	d.BaseDatabase = sqladapter.NewBaseDatabase(d)

	// Binding with sqlbuilder.
	d.SQLBuilder = sqlbuilder.WithSession(d.BaseDatabase, template)

	if err := d.BaseDatabase.BindTx(d.Context(), sqlTx); err != nil {
		return nil, err
	}

	newTx := sqladapter.NewDatabaseTx(d)
	return &tx{DatabaseTx: newTx}, nil
}

// New wraps a regular *sql.DB session and creates a new upper-db session
// backed by it.
func New(sess *sql.DB) (sqlbuilder.Database, error) {
	d := newDatabase(nil)

	// Binding with sqladapter's logic.
	d.BaseDatabase = sqladapter.NewBaseDatabase(d)

	// Binding with sqlbuilder.
	d.SQLBuilder = sqlbuilder.WithSession(d.BaseDatabase, template)

	if err := d.BaseDatabase.BindSession(sess); err != nil {
		return nil, err
	}
	return d, nil
}
