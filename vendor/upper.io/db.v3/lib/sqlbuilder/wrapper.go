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

package sqlbuilder

import (
	"context"
	"database/sql"
	"fmt"
	"sync"

	"upper.io/db.v3"
)

var (
	adapters   map[string]*AdapterFuncMap
	adaptersMu sync.RWMutex
)

func init() {
	adapters = make(map[string]*AdapterFuncMap)
}

// Tx represents a transaction on a SQL database. A transaction is like a
// regular Database except it has two extra methods: Commit and Rollback.
//
// A transaction needs to be committed (with Commit) to make changes permanent,
// changes can be discarded before committing by rolling back (with Rollback).
// After either committing or rolling back a transaction it can not longer be
// used and it's automatically closed.
type Tx interface {
	// All db.Database methods are available on transaction sessions. They will
	// run on the same transaction.
	db.Database

	// All SQLBuilder methods are available on transaction sessions. They will
	// run on the same transaction.
	SQLBuilder

	// db.Tx adds Commit and Rollback methods to the transaction.
	db.Tx

	// Context returns the context used as default for queries on this transaction.
	// If no context has been set, a default context.Background() is returned.
	Context() context.Context

	// WithContext returns a copy of the transaction that uses the given context
	// as default. Copies are safe to use concurrently but they're backed by the
	// same *sql.Tx, so any copy may commit or rollback the parent transaction.
	WithContext(context.Context) Tx
}

// Database represents a SQL database.
type Database interface {
	// All db.Database methods are available on this session.
	db.Database

	// All SQLBuilder methods are available on this session.
	SQLBuilder

	// NewTx creates and returns a transaction that runs on the given context.
	// If a nil context is given, then the transaction will use the session's
	// default context.  The user is responsible for committing or rolling back
	// the session.
	NewTx(ctx context.Context) (Tx, error)

	// Tx creates a new transaction that is passed as argument to the fn
	// function.  The fn function defines a transactional operation.  If the fn
	// function returns nil, the transaction is committed, else the transaction
	// is rolled back.  The transaction session is closed after the function
	// exits, regardless of the error value returned by fn.
	Tx(ctx context.Context, fn func(sess Tx) error) error

	// Context returns the context used as default for queries on this session
	// and for new transactions.  If no context has been set, a default
	// context.Background() is returned.
	Context() context.Context

	// WithContext returns a copy of the session that uses the given context as
	// default. Copies are safe to use concurrently but they're backed by the
	// same *sql.DB. You may close a copy at any point but that won't close the
	// parent session.
	WithContext(context.Context) Database
}

// AdapterFuncMap is a struct that defines a set of functions that adapters
// need to provide.
type AdapterFuncMap struct {
	New   func(sqlDB *sql.DB) (Database, error)
	NewTx func(sqlTx *sql.Tx) (Tx, error)
	Open  func(settings db.ConnectionURL) (Database, error)
}

// RegisterAdapter registers a SQL database adapter. This function must be
// called from adapter packages upon initialization. RegisterAdapter calls
// RegisterAdapter automatically.
func RegisterAdapter(name string, adapter *AdapterFuncMap) {
	adaptersMu.Lock()
	defer adaptersMu.Unlock()

	if name == "" {
		panic(`Missing adapter name`)
	}
	if _, ok := adapters[name]; ok {
		panic(`db.RegisterAdapter() called twice for adapter: ` + name)
	}
	adapters[name] = adapter

	db.RegisterAdapter(name, &db.AdapterFuncMap{
		Open: func(settings db.ConnectionURL) (db.Database, error) {
			return adapter.Open(settings)
		},
	})
}

// adapter returns SQL database functions.
func adapter(name string) AdapterFuncMap {
	adaptersMu.RLock()
	defer adaptersMu.RUnlock()

	if fn, ok := adapters[name]; ok {
		return *fn
	}
	return missingAdapter(name)
}

// Open opens a SQL database.
func Open(adapterName string, settings db.ConnectionURL) (Database, error) {
	return adapter(adapterName).Open(settings)
}

// New wraps an active *sql.DB session and returns a SQLBuilder database.  The
// adapter needs to be imported to the blank namespace in order for it to be
// used here.
//
// This method is internally used by upper-db to create a builder backed by the
// given database.  You may want to use your adapter's New function instead of
// this one.
func New(adapterName string, sqlDB *sql.DB) (Database, error) {
	return adapter(adapterName).New(sqlDB)
}

// NewTx wraps an active *sql.Tx transation and returns a SQLBuilder
// transaction.  The adapter needs to be imported to the blank namespace in
// order for it to be used.
//
// This method is internally used by upper-db to create a builder backed by the
// given transaction.  You may want to use your adapter's NewTx function
// instead of this one.
func NewTx(adapterName string, sqlTx *sql.Tx) (Tx, error) {
	return adapter(adapterName).NewTx(sqlTx)
}

func missingAdapter(name string) AdapterFuncMap {
	err := fmt.Errorf("upper: Missing SQL adapter %q, forgot to import?", name)
	return AdapterFuncMap{
		New: func(*sql.DB) (Database, error) {
			return nil, err
		},
		NewTx: func(*sql.Tx) (Tx, error) {
			return nil, err
		},
		Open: func(db.ConnectionURL) (Database, error) {
			return nil, err
		},
	}
}
