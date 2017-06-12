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

package sqlite // import "upper.io/db.v3/sqlite"

import (
	"database/sql"

	"upper.io/db.v3"

	"upper.io/db.v3/internal/sqladapter"
	"upper.io/db.v3/lib/sqlbuilder"
)

const sqlDriver = `sqlite`

// Adapter is the public name of the adapter.
const Adapter = sqlDriver

func init() {
	sqlbuilder.RegisterAdapter(Adapter, &sqlbuilder.AdapterFuncMap{
		New:   New,
		NewTx: NewTx,
		Open:  Open,
	})
}

// Open stablishes a new connection with the SQL server.
func Open(settings db.ConnectionURL) (sqlbuilder.Database, error) {
	d := newDatabase(settings)
	if err := d.Open(settings); err != nil {
		return nil, err
	}
	return d, nil
}

// NewTx returns a transaction session.
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

// New wraps the given *sql.DB session and creates a new db session.
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
