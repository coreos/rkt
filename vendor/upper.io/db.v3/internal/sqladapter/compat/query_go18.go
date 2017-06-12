// +build go1.8

package compat

import (
	"context"
	"database/sql"
)

type PreparedExecer interface {
	ExecContext(context.Context, ...interface{}) (sql.Result, error)
}

func PreparedExecContext(p PreparedExecer, ctx context.Context, args []interface{}) (sql.Result, error) {
	return p.ExecContext(ctx, args...)
}

type Execer interface {
	ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
}

func ExecContext(p Execer, ctx context.Context, query string, args []interface{}) (sql.Result, error) {
	return p.ExecContext(ctx, query, args...)
}

type PreparedQueryer interface {
	QueryContext(context.Context, ...interface{}) (*sql.Rows, error)
}

func PreparedQueryContext(p PreparedQueryer, ctx context.Context, args []interface{}) (*sql.Rows, error) {
	return p.QueryContext(ctx, args...)
}

type Queryer interface {
	QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error)
}

func QueryContext(p Queryer, ctx context.Context, query string, args []interface{}) (*sql.Rows, error) {
	return p.QueryContext(ctx, query, args...)
}

type PreparedRowQueryer interface {
	QueryRowContext(context.Context, ...interface{}) *sql.Row
}

func PreparedQueryRowContext(p PreparedRowQueryer, ctx context.Context, args []interface{}) *sql.Row {
	return p.QueryRowContext(ctx, args...)
}

type RowQueryer interface {
	QueryRowContext(context.Context, string, ...interface{}) *sql.Row
}

func QueryRowContext(p RowQueryer, ctx context.Context, query string, args []interface{}) *sql.Row {
	return p.QueryRowContext(ctx, query, args...)
}

type Preparer interface {
	PrepareContext(context.Context, string) (*sql.Stmt, error)
}

func PrepareContext(p Preparer, ctx context.Context, query string) (*sql.Stmt, error) {
	return p.PrepareContext(ctx, query)
}

type TxStarter interface {
	BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error)
}

func BeginTx(p TxStarter, ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return p.BeginTx(ctx, opts)
}
