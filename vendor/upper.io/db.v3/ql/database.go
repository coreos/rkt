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

// Package ql wraps the github.com/cznic/ql/driver QL driver. See
// https://upper.io/db.v3/ql for documentation, particularities and usage
// examples.
package ql

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"sync/atomic"

	_ "github.com/cznic/ql/driver" // QL driver
	"upper.io/db.v3"
	"upper.io/db.v3/internal/sqladapter"
	"upper.io/db.v3/internal/sqladapter/compat"
	"upper.io/db.v3/internal/sqladapter/exql"
	"upper.io/db.v3/lib/sqlbuilder"
)

// database is the actual implementation of Database
type database struct {
	sqladapter.BaseDatabase // Leveraged by sqladapter
	sqlbuilder.SQLBuilder

	connURL db.ConnectionURL
	mu      sync.Mutex
}

var (
	fileOpenCount       int32
	errTooManyOpenFiles       = errors.New(`Too many open database files.`)
	maxOpenFiles        int32 = 5
)

var (
	_ = sqlbuilder.Database(&database{})
	_ = sqladapter.Database(&database{})
)

// newDatabase binds *database with sqladapter and the SQL builer.
func newDatabase(settings db.ConnectionURL) *database {
	return &database{
		connURL: settings,
	}
}

// Open stablishes a new connection to a SQL server.
func Open(settings db.ConnectionURL) (sqlbuilder.Database, error) {
	d := newDatabase(settings)
	if err := d.Open(settings); err != nil {
		return nil, err
	}
	return d, nil
}

// CleanUp cleans up the session.
func (d *database) CleanUp() error {
	if atomic.AddInt32(&fileOpenCount, -1) < 0 {
		return errors.New(`Close() without Open()?`)
	}
	return nil
}

// ConnectionURL returns this database's ConnectionURL.
func (d *database) ConnectionURL() db.ConnectionURL {
	return d.connURL
}

// Open stablishes a new connection with the SQL server.
func (d *database) Open(connURL db.ConnectionURL) error {
	if connURL == nil {
		return db.ErrMissingConnURL
	}
	d.connURL = connURL
	return d.open()
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

// NewTx starts a transaction block.
func (d *database) NewTx(ctx context.Context) (sqlbuilder.Tx, error) {
	if ctx == nil {
		ctx = d.Context()
	}
	nTx, err := d.NewDatabaseTx(ctx)
	if err != nil {
		return nil, err
	}
	return &tx{DatabaseTx: nTx}, nil
}

// Collections returns a list of non-system tables from the database.
func (d *database) Collections() (collections []string, err error) {
	q := d.Select("Name").
		From("__Table")

	iter := q.Iterator()
	defer iter.Close()

	for iter.Next() {
		var tableName string
		if err := iter.Scan(&tableName); err != nil {
			return nil, err
		}
		collections = append(collections, tableName)
	}

	return collections, nil
}

func (d *database) open() error {
	// Binding with sqladapter's logic.
	d.BaseDatabase = sqladapter.NewBaseDatabase(d)

	// Binding with sqlbuilder.
	d.SQLBuilder = sqlbuilder.WithSession(d.BaseDatabase, template)

	openFn := func() error {
		openFiles := atomic.LoadInt32(&fileOpenCount)
		if openFiles < maxOpenFiles {
			sess, err := sql.Open("ql", d.ConnectionURL().String())
			if err == nil {
				if err := d.BaseDatabase.BindSession(sess); err != nil {
					return err
				}
				atomic.AddInt32(&fileOpenCount, 1)
				return nil
			}
			return err
		}
		return errTooManyOpenFiles
	}

	if err := d.BaseDatabase.WaitForConnection(openFn); err != nil {
		return err
	}

	return nil
}

func (d *database) clone(ctx context.Context, checkConn bool) (*database, error) {
	clone := newDatabase(d.connURL)

	var err error
	clone.BaseDatabase, err = d.NewClone(clone, checkConn)
	if err != nil {
		return nil, err
	}

	clone.SetContext(ctx)

	clone.SQLBuilder = sqlbuilder.WithSession(clone.BaseDatabase, template)

	return clone, nil
}

// CompileStatement allows sqladapter to compile the given statement into the
// format SQLite expects.
func (d *database) CompileStatement(stmt *exql.Statement, args []interface{}) (string, []interface{}) {
	compiled, err := stmt.Compile(template)
	if err != nil {
		panic(err.Error())
	}
	query, args := sqlbuilder.Preprocess(compiled, args)
	return sqladapter.ReplaceWithDollarSign(query), args
}

// Err allows sqladapter to translate some known errors into generic errors.
func (d *database) Err(err error) error {
	if err != nil {
		if err == errTooManyOpenFiles {
			return db.ErrTooManyClients
		}
	}
	return err
}

// StatementExec wraps the statement to execute around a transaction.
func (d *database) StatementExec(ctx context.Context, query string, args ...interface{}) (res sql.Result, err error) {
	if d.Transaction() != nil {
		return compat.ExecContext(d.Driver().(*sql.Tx), ctx, query, args)
	}

	sqlTx, err := compat.BeginTx(d.Session(), ctx, nil)
	if err != nil {
		return nil, err
	}

	if res, err = compat.ExecContext(sqlTx, ctx, query, args); err != nil {
		return nil, err
	}

	if err = sqlTx.Commit(); err != nil {
		return nil, err
	}

	return res, err
}

// NewCollection allows sqladapter create a local db.Collection.
func (d *database) NewCollection(name string) db.Collection {
	return newTable(d, name)
}

// Tx creates a transaction and passes it to the given function, if if the
// function returns no error then the transaction is commited.
func (d *database) Tx(ctx context.Context, fn func(tx sqlbuilder.Tx) error) error {
	return sqladapter.RunTx(d, ctx, fn)
}

// NewDatabaseTx allows sqladapter start a transaction block.
func (d *database) NewDatabaseTx(ctx context.Context) (sqladapter.DatabaseTx, error) {
	clone, err := d.clone(ctx, true)
	if err != nil {
		return nil, err
	}
	clone.mu.Lock()
	defer clone.mu.Unlock()

	openFn := func() error {
		sqlTx, err := compat.BeginTx(clone.BaseDatabase.Session(), ctx, nil)
		if err == nil {
			return clone.BindTx(ctx, sqlTx)
		}
		return err
	}

	if err := d.BaseDatabase.WaitForConnection(openFn); err != nil {
		return nil, err
	}

	return sqladapter.NewDatabaseTx(clone), nil
}

// LookupName allows sqladapter look up the database's name.
func (d *database) LookupName() (string, error) {
	connURL, err := ParseURL(d.ConnectionURL().String())
	if err != nil {
		return "", err
	}
	return connURL.Database, nil
}

// TableExists allows sqladapter check whether a table exists and returns an
// error in case it doesn't.
func (d *database) TableExists(name string) error {
	q := d.SQLBuilder.Select("Name").
		From("__Table").
		Where("Name == ?", name)

	iter := q.Iterator()
	defer iter.Close()

	if iter.Next() {
		var name string
		if err := iter.Scan(&name); err != nil {
			return err
		}
		return nil
	}
	return db.ErrCollectionDoesNotExist
}

// PrimaryKeys allows sqladapter find a table's primary keys.
func (d *database) PrimaryKeys(tableName string) ([]string, error) {
	return []string{"id()"}, nil
}

// WithContext creates a copy of the session on the given context.
func (d *database) WithContext(ctx context.Context) sqlbuilder.Database {
	newDB, _ := d.clone(ctx, false)
	return newDB
}
