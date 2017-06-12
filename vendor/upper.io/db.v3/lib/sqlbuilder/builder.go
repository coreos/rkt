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

// Package sqlbuilder provides tools for building custom SQL queries.
package sqlbuilder

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"upper.io/db.v3"
	"upper.io/db.v3/internal/sqladapter/compat"
	"upper.io/db.v3/internal/sqladapter/exql"
	"upper.io/db.v3/lib/reflectx"
)

// MapOptions represents options for the mapper.
type MapOptions struct {
	IncludeZeroed bool
	IncludeNil    bool
}

var defaultMapOptions = MapOptions{
	IncludeZeroed: false,
	IncludeNil:    false,
}

type compilable interface {
	Compile() (string, error)
	Arguments() []interface{}
}

type hasIsZero interface {
	IsZero() bool
}

type hasArguments interface {
	Arguments() []interface{}
}

type hasStatement interface {
	statement() *exql.Statement
}

type iterator struct {
	cursor *sql.Rows // This is the main query cursor. It starts as a nil value.
	err    error
}

type fieldValue struct {
	fields []string
	values []interface{}
}

var (
	reInvisibleChars       = regexp.MustCompile(`[\s\r\n\t]+`)
	reColumnCompareExclude = regexp.MustCompile(`[^a-zA-Z0-9]`)
)

var (
	sqlPlaceholder = exql.RawValue(`?`)
)

type exprDB interface {
	StatementPrepare(ctx context.Context, stmt *exql.Statement) (*sql.Stmt, error)
	StatementQuery(ctx context.Context, stmt *exql.Statement, args ...interface{}) (*sql.Rows, error)
	StatementQueryRow(ctx context.Context, stmt *exql.Statement, args ...interface{}) (*sql.Row, error)
	StatementExec(ctx context.Context, stmt *exql.Statement, args ...interface{}) (sql.Result, error)

	Context() context.Context
}

type sqlBuilder struct {
	sess exprDB
	t    *templateWithUtils
}

// WithSession returns a query builder that is bound to the given database session.
func WithSession(sess interface{}, t *exql.Template) SQLBuilder {
	if sqlDB, ok := sess.(*sql.DB); ok {
		sess = sqlDB
	}
	return &sqlBuilder{
		sess: sess.(exprDB), // Let it panic, it will show the developer an informative error.
		t:    newTemplateWithUtils(t),
	}
}

// WithTemplate returns a builder that is based on the given template.
func WithTemplate(t *exql.Template) SQLBuilder {
	return &sqlBuilder{
		t: newTemplateWithUtils(t),
	}
}

// NewIterator creates an iterator using the given *sql.Rows.
func NewIterator(rows *sql.Rows) Iterator {
	return &iterator{rows, nil}
}

func (b *sqlBuilder) Iterator(query interface{}, args ...interface{}) Iterator {
	return b.IteratorContext(b.sess.Context(), query, args...)
}

func (b *sqlBuilder) IteratorContext(ctx context.Context, query interface{}, args ...interface{}) Iterator {
	rows, err := b.QueryContext(ctx, query, args...)
	return &iterator{rows, err}
}

func (b *sqlBuilder) Prepare(query interface{}) (*sql.Stmt, error) {
	return b.PrepareContext(b.sess.Context(), query)
}

func (b *sqlBuilder) PrepareContext(ctx context.Context, query interface{}) (*sql.Stmt, error) {
	switch q := query.(type) {
	case *exql.Statement:
		return b.sess.StatementPrepare(ctx, q)
	case string:
		return b.sess.StatementPrepare(ctx, exql.RawSQL(q))
	case db.RawValue:
		return b.PrepareContext(ctx, q.Raw())
	default:
		return nil, fmt.Errorf("Unsupported query type %T.", query)
	}
}

func (b *sqlBuilder) Exec(query interface{}, args ...interface{}) (sql.Result, error) {
	return b.ExecContext(b.sess.Context(), query, args...)
}

func (b *sqlBuilder) ExecContext(ctx context.Context, query interface{}, args ...interface{}) (sql.Result, error) {
	switch q := query.(type) {
	case *exql.Statement:
		return b.sess.StatementExec(ctx, q, args...)
	case string:
		return b.sess.StatementExec(ctx, exql.RawSQL(q), args...)
	case db.RawValue:
		return b.ExecContext(ctx, q.Raw(), q.Arguments()...)
	default:
		return nil, fmt.Errorf("Unsupported query type %T.", query)
	}
}

func (b *sqlBuilder) Query(query interface{}, args ...interface{}) (*sql.Rows, error) {
	return b.QueryContext(b.sess.Context(), query, args...)
}

func (b *sqlBuilder) QueryContext(ctx context.Context, query interface{}, args ...interface{}) (*sql.Rows, error) {
	switch q := query.(type) {
	case *exql.Statement:
		return b.sess.StatementQuery(ctx, q, args...)
	case string:
		return b.sess.StatementQuery(ctx, exql.RawSQL(q), args...)
	case db.RawValue:
		return b.QueryContext(ctx, q.Raw(), q.Arguments()...)
	default:
		return nil, fmt.Errorf("Unsupported query type %T.", query)
	}
}

func (b *sqlBuilder) QueryRow(query interface{}, args ...interface{}) (*sql.Row, error) {
	return b.QueryRowContext(b.sess.Context(), query, args...)
}

func (b *sqlBuilder) QueryRowContext(ctx context.Context, query interface{}, args ...interface{}) (*sql.Row, error) {
	switch q := query.(type) {
	case *exql.Statement:
		return b.sess.StatementQueryRow(ctx, q, args...)
	case string:
		return b.sess.StatementQueryRow(ctx, exql.RawSQL(q), args...)
	case db.RawValue:
		return b.QueryRowContext(ctx, q.Raw(), q.Arguments()...)
	default:
		return nil, fmt.Errorf("Unsupported query type %T.", query)
	}
}

func (b *sqlBuilder) SelectFrom(table ...interface{}) Selector {
	qs := &selector{
		builder: b,
	}
	return qs.From(table...)
}

func (b *sqlBuilder) Select(columns ...interface{}) Selector {
	qs := &selector{
		builder: b,
	}
	return qs.Columns(columns...)
}

func (b *sqlBuilder) InsertInto(table string) Inserter {
	qi := &inserter{
		builder: b,
	}
	return qi.Into(table)
}

func (b *sqlBuilder) DeleteFrom(table string) Deleter {
	qd := &deleter{
		builder: b,
	}
	return qd.setTable(table)
}

func (b *sqlBuilder) Update(table string) Updater {
	qu := &updater{
		builder: b,
	}
	return qu.setTable(table)
}

// Map receives a pointer to map or struct and maps it to columns and values.
func Map(item interface{}, options *MapOptions) ([]string, []interface{}, error) {
	var fv fieldValue
	if options == nil {
		options = &defaultMapOptions
	}

	itemV := reflect.ValueOf(item)
	if !itemV.IsValid() {
		return nil, nil, nil
	}

	itemT := itemV.Type()

	if itemT.Kind() == reflect.Ptr {
		// Single dereference. Just in case the user passes a pointer to struct
		// instead of a struct.
		item = itemV.Elem().Interface()
		itemV = reflect.ValueOf(item)
		itemT = itemV.Type()
	}

	switch itemT.Kind() {
	case reflect.Struct:
		fieldMap := mapper.TypeMap(itemT).Names
		nfields := len(fieldMap)

		fv.values = make([]interface{}, 0, nfields)
		fv.fields = make([]string, 0, nfields)

		for _, fi := range fieldMap {

			// Field options
			_, tagOmitEmpty := fi.Options["omitempty"]
			_, tagStringArray := fi.Options["stringarray"]
			_, tagInt64Array := fi.Options["int64array"]
			_, tagJSONB := fi.Options["jsonb"]

			fld := reflectx.FieldByIndexesReadOnly(itemV, fi.Index)
			if fld.Kind() == reflect.Ptr && fld.IsNil() {
				if tagOmitEmpty && !options.IncludeNil {
					continue
				}
				fv.fields = append(fv.fields, fi.Name)
				if tagOmitEmpty {
					fv.values = append(fv.values, sqlDefault)
				} else {
					fv.values = append(fv.values, nil)
				}
				continue
			}

			var value interface{}
			switch {
			case tagStringArray:
				v, ok := fld.Interface().([]string)
				if !ok {
					return nil, nil, fmt.Errorf(`Expecting field %q to be []string (using "stringarray" tag)`, fi.Name)
				}
				value = stringArray(v)
			case tagInt64Array:
				v, ok := fld.Interface().([]int64)
				if !ok {
					return nil, nil, fmt.Errorf(`Expecting field %q to be []int64 (using "int64array" tag)`, fi.Name)
				}
				value = int64Array(v)
			case tagJSONB:
				value = jsonbType{fld.Interface()}
			default:
				value = fld.Interface()
			}

			isZero := false
			if t, ok := fld.Interface().(hasIsZero); ok {
				if t.IsZero() {
					isZero = true
				}
			} else if fld.Kind() == reflect.Array || fld.Kind() == reflect.Slice {
				if fld.Len() == 0 {
					isZero = true
				}
			} else if reflect.DeepEqual(fi.Zero.Interface(), value) {
				isZero = true
			}

			if isZero && tagOmitEmpty && !options.IncludeZeroed {
				continue
			}

			fv.fields = append(fv.fields, fi.Name)
			v, err := marshal(value)
			if err != nil {
				return nil, nil, err
			}
			if isZero && tagOmitEmpty {
				v = sqlDefault
			}
			fv.values = append(fv.values, v)
		}

	case reflect.Map:
		nfields := itemV.Len()
		fv.values = make([]interface{}, nfields)
		fv.fields = make([]string, nfields)
		mkeys := itemV.MapKeys()

		for i, keyV := range mkeys {
			valv := itemV.MapIndex(keyV)
			fv.fields[i] = fmt.Sprintf("%v", keyV.Interface())

			v, err := marshal(valv.Interface())
			if err != nil {
				return nil, nil, err
			}

			fv.values[i] = v
		}
	default:
		return nil, nil, ErrExpectingPointerToEitherMapOrStruct
	}

	sort.Sort(&fv)

	return fv.fields, fv.values, nil
}

func extractArguments(fragments []interface{}) []interface{} {
	args := []interface{}{}
	l := len(fragments)
	for i := 0; i < l; i++ {
		switch v := fragments[i].(type) {
		case hasArguments: // TODO: use this on other places where we want to extract arguments.
			args = append(args, v.Arguments()...)
		}
	}
	return args
}

func columnFragments(columns []interface{}) ([]exql.Fragment, []interface{}, error) {
	l := len(columns)
	f := make([]exql.Fragment, l)
	args := []interface{}{}

	for i := 0; i < l; i++ {
		switch v := columns[i].(type) {
		case compilable:
			c, err := v.Compile()
			if err != nil {
				return nil, nil, err
			}
			q, a := Preprocess(c, v.Arguments())
			f[i] = exql.RawValue(q)
			args = append(args, a...)
		case db.Function:
			fnName, fnArgs := v.Name(), v.Arguments()
			if len(fnArgs) == 0 {
				fnName = fnName + "()"
			} else {
				fnName = fnName + "(?" + strings.Repeat("?, ", len(fnArgs)-1) + ")"
			}
			fnName, fnArgs = Preprocess(fnName, fnArgs)
			f[i] = exql.RawValue(fnName)
			args = append(args, fnArgs...)
		case db.RawValue:
			q, a := Preprocess(v.Raw(), v.Arguments())
			f[i] = exql.RawValue(q)
			args = append(args, a...)
		case exql.Fragment:
			f[i] = v
		case string:
			f[i] = exql.ColumnWithName(v)
		case int:
			f[i] = exql.RawValue(fmt.Sprintf("%v", v))
		case interface{}:
			f[i] = exql.ColumnWithName(fmt.Sprintf("%v", v))
		default:
			return nil, nil, fmt.Errorf("Unexpected argument type %T for Select() argument.", v)
		}
	}
	return f, args, nil
}

func prepareQueryForDisplay(in string) (out string) {
	j := 1
	for i := range in {
		if in[i] == '?' {
			out = out + "$" + strconv.Itoa(j)
			j++
		} else {
			out = out + string(in[i])
		}
	}

	out = reInvisibleChars.ReplaceAllString(out, ` `)
	return strings.TrimSpace(out)
}

func (iter *iterator) NextScan(dst ...interface{}) error {
	if ok := iter.Next(); ok {
		return iter.Scan(dst...)
	}
	if err := iter.Err(); err != nil {
		return err
	}
	return db.ErrNoMoreRows
}

func (iter *iterator) ScanOne(dst ...interface{}) error {
	defer iter.Close()
	return iter.NextScan(dst...)
}

func (iter *iterator) Scan(dst ...interface{}) error {
	if err := iter.Err(); err != nil {
		return err
	}
	return iter.cursor.Scan(dst...)
}

func (iter *iterator) setErr(err error) error {
	iter.err = err
	return iter.err
}

func (iter *iterator) One(dst interface{}) error {
	if err := iter.Err(); err != nil {
		return err
	}
	defer iter.Close()
	return iter.setErr(iter.next(dst))
}

func (iter *iterator) All(dst interface{}) error {
	if err := iter.Err(); err != nil {
		return err
	}
	defer iter.Close()

	// Fetching all results within the cursor.
	if err := fetchRows(iter.cursor, dst); err != nil {
		return iter.setErr(err)
	}

	return nil
}

func (iter *iterator) Err() (err error) {
	return iter.err
}

func (iter *iterator) Next(dst ...interface{}) bool {
	if err := iter.Err(); err != nil {
		return false
	}

	if err := iter.next(dst...); err != nil {
		// ignore db.ErrNoMoreRows, just break.
		if err != db.ErrNoMoreRows {
			iter.setErr(err)
		}
		return false
	}

	return true
}

func (iter *iterator) next(dst ...interface{}) error {
	if iter.cursor == nil {
		return iter.setErr(db.ErrNoMoreRows)
	}

	switch len(dst) {
	case 0:
		if ok := iter.cursor.Next(); !ok {
			defer iter.Close()
			err := iter.cursor.Err()
			if err == nil {
				err = db.ErrNoMoreRows
			}
			return err
		}
		return nil
	case 1:
		if err := fetchRow(iter.cursor, dst[0]); err != nil {
			defer iter.Close()
			return err
		}
		return nil
	}

	return errors.New("Next does not currently supports more than one parameters")
}

func (iter *iterator) Close() (err error) {
	if iter.cursor != nil {
		err = iter.cursor.Close()
		iter.cursor = nil
	}
	return err
}

func marshal(v interface{}) (interface{}, error) {
	if m, isMarshaler := v.(db.Marshaler); isMarshaler {
		var err error
		if v, err = m.MarshalDB(); err != nil {
			return nil, err
		}
	}
	return v, nil
}

func (fv *fieldValue) Len() int {
	return len(fv.fields)
}

func (fv *fieldValue) Swap(i, j int) {
	fv.fields[i], fv.fields[j] = fv.fields[j], fv.fields[i]
	fv.values[i], fv.values[j] = fv.values[j], fv.values[i]
}

func (fv *fieldValue) Less(i, j int) bool {
	return fv.fields[i] < fv.fields[j]
}

type exprProxy struct {
	db *sql.DB
	t  *exql.Template
}

func newSqlgenProxy(db *sql.DB, t *exql.Template) *exprProxy {
	return &exprProxy{db: db, t: t}
}

func (p *exprProxy) Context() context.Context {
	log.Printf("Missing context")
	return context.Background()
}

func (p *exprProxy) StatementExec(ctx context.Context, stmt *exql.Statement, args ...interface{}) (sql.Result, error) {
	s, err := stmt.Compile(p.t)
	if err != nil {
		return nil, err
	}
	return compat.ExecContext(p.db, ctx, s, args)
}

func (p *exprProxy) StatementPrepare(ctx context.Context, stmt *exql.Statement) (*sql.Stmt, error) {
	s, err := stmt.Compile(p.t)
	if err != nil {
		return nil, err
	}
	return compat.PrepareContext(p.db, ctx, s)
}

func (p *exprProxy) StatementQuery(ctx context.Context, stmt *exql.Statement, args ...interface{}) (*sql.Rows, error) {
	s, err := stmt.Compile(p.t)
	if err != nil {
		return nil, err
	}
	return compat.QueryContext(p.db, ctx, s, args)
}

func (p *exprProxy) StatementQueryRow(ctx context.Context, stmt *exql.Statement, args ...interface{}) (*sql.Row, error) {
	s, err := stmt.Compile(p.t)
	if err != nil {
		return nil, err
	}
	return compat.QueryRowContext(p.db, ctx, s, args), nil
}

var (
	_ = SQLBuilder(&sqlBuilder{})
	_ = exprDB(&exprProxy{})
)

func joinArguments(args ...[]interface{}) []interface{} {
	total := 0
	for i := range args {
		total += len(args[i])
	}
	if total == 0 {
		return nil
	}

	flatten := make([]interface{}, 0, total)
	for i := range args {
		flatten = append(flatten, args[i]...)
	}
	return flatten
}
