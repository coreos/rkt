// +build !go1.8

package compat

import (
	"context"
	"database/sql"
)

type PreparedExecer interface {
	Exec(...interface{}) (sql.Result, error)
}

func PreparedExecContext(p PreparedExecer, ctx context.Context, args []interface{}) (sql.Result, error) {
	return p.Exec(args...)
}

type Execer interface {
	Exec(string, ...interface{}) (sql.Result, error)
}

func ExecContext(p Execer, ctx context.Context, query string, args []interface{}) (sql.Result, error) {
	return p.Exec(query, args...)
}

type PreparedQueryer interface {
	Query(...interface{}) (*sql.Rows, error)
}

func PreparedQueryContext(p PreparedQueryer, ctx context.Context, args []interface{}) (*sql.Rows, error) {
	return p.Query(args...)
}

type Queryer interface {
	Query(string, ...interface{}) (*sql.Rows, error)
}

func QueryContext(p Queryer, ctx context.Context, query string, args []interface{}) (*sql.Rows, error) {
	return p.Query(query, args...)
}

type PreparedRowQueryer interface {
	QueryRow(...interface{}) *sql.Row
}

func PreparedQueryRowContext(p PreparedRowQueryer, ctx context.Context, args []interface{}) *sql.Row {
	return p.QueryRow(args...)
}

type RowQueryer interface {
	QueryRow(string, ...interface{}) *sql.Row
}

func QueryRowContext(p RowQueryer, ctx context.Context, query string, args []interface{}) *sql.Row {
	return p.QueryRow(query, args...)
}

type Preparer interface {
	Prepare(string) (*sql.Stmt, error)
}

func PrepareContext(p Preparer, ctx context.Context, query string) (*sql.Stmt, error) {
	return p.Prepare(query)
}

type TxStarter interface {
	Begin() (*sql.Tx, error)
}

func BeginTx(p TxStarter, ctx context.Context, opts interface{}) (*sql.Tx, error) {
	return p.Begin()
}
