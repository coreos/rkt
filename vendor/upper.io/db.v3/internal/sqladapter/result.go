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

package sqladapter

import (
	"sync"
	"sync/atomic"

	"upper.io/db.v3"
	"upper.io/db.v3/internal/immutable"
	"upper.io/db.v3/lib/sqlbuilder"
)

type Result struct {
	builder sqlbuilder.SQLBuilder

	err atomic.Value

	iter   sqlbuilder.Iterator
	iterMu sync.Mutex

	prev *Result
	fn   func(*result) error
}

// result represents a delimited set of items bound by a condition.
type result struct {
	table   string
	limit   int
	offset  int
	fields  []interface{}
	columns []interface{}
	orderBy []interface{}
	groupBy []interface{}
	conds   [][]interface{}
}

func filter(conds []interface{}) []interface{} {
	return conds
}

// NewResult creates and Results a new Result set on the given table, this set
// is limited by the given exql.Where conditions.
func NewResult(builder sqlbuilder.SQLBuilder, table string, conds []interface{}) *Result {
	r := &Result{
		builder: builder,
	}
	return r.from(table).where(conds)
}

func (r *Result) frame(fn func(*result) error) *Result {
	return &Result{prev: r, fn: fn}
}

func (r *Result) SQLBuilder() sqlbuilder.SQLBuilder {
	if r.prev == nil {
		return r.builder
	}
	return r.prev.SQLBuilder()
}

func (r *Result) from(table string) *Result {
	return r.frame(func(res *result) error {
		res.table = table
		return nil
	})
}

func (r *Result) where(conds []interface{}) *Result {
	return r.frame(func(res *result) error {
		res.conds = [][]interface{}{conds}
		return nil
	})
}

func (r *Result) setErr(err error) error {
	if err == nil {
		return nil
	}
	r.err.Store(err)
	return err
}

// Err returns the last error that has happened with the result set,
// nil otherwise
func (r *Result) Err() error {
	if errV := r.err.Load(); errV != nil {
		return errV.(error)
	}
	return nil
}

// Where sets conditions for the result set.
func (r *Result) Where(conds ...interface{}) db.Result {
	return r.where(conds)
}

// And adds more conditions on top of the existing ones.
func (r *Result) And(conds ...interface{}) db.Result {
	return r.frame(func(res *result) error {
		res.conds = append(res.conds, conds)
		return nil
	})
}

// Limit determines the maximum limit of Results to be returned.
func (r *Result) Limit(n int) db.Result {
	return r.frame(func(res *result) error {
		res.limit = n
		return nil
	})
}

// Offset determines how many documents will be skipped before starting to grab
// Results.
func (r *Result) Offset(n int) db.Result {
	return r.frame(func(res *result) error {
		res.offset = n
		return nil
	})
}

// Group is used to group Results that have the same value in the same column
// or columns.
func (r *Result) Group(fields ...interface{}) db.Result {
	return r.frame(func(res *result) error {
		res.groupBy = fields
		return nil
	})
}

// OrderBy determines sorting of Results according to the provided names. Fields
// may be prefixed by - (minus) which means descending order, ascending order
// would be used otherwise.
func (r *Result) OrderBy(fields ...interface{}) db.Result {
	return r.frame(func(res *result) error {
		res.orderBy = fields
		return nil
	})
}

// Select determines which fields to return.
func (r *Result) Select(fields ...interface{}) db.Result {
	return r.frame(func(res *result) error {
		res.fields = fields
		return nil
	})
}

// String satisfies fmt.Stringer
func (r *Result) String() string {
	query, err := r.buildSelect()
	if err != nil {
		panic(err.Error())
	}
	return query.String()
}

// All dumps all Results into a pointer to an slice of structs or maps.
func (r *Result) All(dst interface{}) error {
	query, err := r.buildSelect()
	if err != nil {
		return r.setErr(err)
	}
	err = query.Iterator().All(dst)
	return r.setErr(err)
}

// One fetches only one Result from the set.
func (r *Result) One(dst interface{}) error {
	query, err := r.buildSelect()
	if err != nil {
		return r.setErr(err)
	}
	err = query.Iterator().One(dst)
	return r.setErr(err)
}

// Next fetches the next Result from the set.
func (r *Result) Next(dst interface{}) bool {
	r.iterMu.Lock()
	defer r.iterMu.Unlock()

	if r.iter == nil {
		query, err := r.buildSelect()
		if err != nil {
			r.setErr(err)
			return false
		}
		r.iter = query.Iterator()
	}

	if r.iter.Next(dst) {
		return true
	}

	if err := r.iter.Err(); err != db.ErrNoMoreRows {
		r.setErr(err)
	}

	return false
}

// Delete deletes all matching items from the collection.
func (r *Result) Delete() error {
	query, err := r.buildDelete()
	if err != nil {
		return r.setErr(err)
	}

	_, err = query.Exec()
	return r.setErr(err)
}

// Close closes the Result set.
func (r *Result) Close() error {
	if r.iter != nil {
		return r.setErr(r.iter.Close())
	}
	return nil
}

// Update updates matching items from the collection with values of the given
// map or struct.
func (r *Result) Update(values interface{}) error {
	query, err := r.buildUpdate(values)
	if err != nil {
		return r.setErr(err)
	}

	_, err = query.Exec()
	return r.setErr(err)
}

// Count counts the elements on the set.
func (r *Result) Count() (uint64, error) {
	query, err := r.buildCount()
	if err != nil {
		return 0, r.setErr(err)
	}

	counter := struct {
		Count uint64 `db:"_t"`
	}{}
	if err := query.Iterator().One(&counter); err != nil {
		if err == db.ErrNoMoreRows {
			return 0, nil
		}
		return 0, r.setErr(err)
	}

	return counter.Count, nil
}

func (r *Result) buildSelect() (sqlbuilder.Selector, error) {
	if err := r.Err(); err != nil {
		return nil, err
	}

	res, err := r.fastForward()
	if err != nil {
		return nil, err
	}

	sel := r.SQLBuilder().Select(res.fields...).
		From(res.table).
		Limit(res.limit).
		Offset(res.offset).
		GroupBy(res.groupBy...).
		OrderBy(res.orderBy...)

	for i := range res.conds {
		sel = sel.And(filter(res.conds[i])...)
	}

	return sel, nil
}

func (r *Result) buildDelete() (sqlbuilder.Deleter, error) {
	if err := r.Err(); err != nil {
		return nil, err
	}

	res, err := r.fastForward()
	if err != nil {
		return nil, err
	}

	del := r.SQLBuilder().DeleteFrom(res.table).
		Limit(res.limit)

	for i := range res.conds {
		del = del.And(filter(res.conds[i])...)
	}

	return del, nil
}

func (r *Result) buildUpdate(values interface{}) (sqlbuilder.Updater, error) {
	if err := r.Err(); err != nil {
		return nil, err
	}

	res, err := r.fastForward()
	if err != nil {
		return nil, err
	}

	upd := r.SQLBuilder().Update(res.table).
		Set(values).
		Limit(res.limit)

	for i := range res.conds {
		upd = upd.And(filter(res.conds[i])...)
	}

	return upd, nil
}

func (r *Result) buildCount() (sqlbuilder.Selector, error) {
	if err := r.Err(); err != nil {
		return nil, err
	}

	res, err := r.fastForward()
	if err != nil {
		return nil, err
	}

	sel := r.SQLBuilder().Select(db.Raw("count(1) AS _t")).
		From(res.table).
		GroupBy(res.groupBy...)

	for i := range res.conds {
		sel = sel.And(filter(res.conds[i])...)
	}

	return sel, nil
}

func (r *Result) Prev() immutable.Immutable {
	if r == nil {
		return nil
	}
	return r.prev
}

func (r *Result) Fn(in interface{}) error {
	if r.fn == nil {
		return nil
	}
	return r.fn(in.(*result))
}

func (r *Result) Base() interface{} {
	return &result{}
}

func (r *Result) fastForward() (*result, error) {
	ff, err := immutable.FastForward(r)
	if err != nil {
		return nil, err
	}
	return ff.(*result), nil
}

var _ = immutable.Immutable(&Result{})
