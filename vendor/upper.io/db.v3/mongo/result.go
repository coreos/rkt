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

package mongo

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"encoding/json"

	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"upper.io/db.v3"
)

// result represents a query result.
type result struct {
	c           *Collection
	queryChunks *chunks
	iter        *mgo.Iter
	errMu       sync.RWMutex
	err         error
}

var (
	errUnknownSortValue = errors.New(`Unknown sort value "%s".`)
)

func (r *result) setErr(err error) error {
	r.errMu.Lock()
	defer r.errMu.Unlock()

	if err != nil {
		r.err = err
	}
	return err
}

func (r *result) Err() error {
	r.errMu.RLock()
	defer r.errMu.RUnlock()

	return r.err
}

// setCursor creates a *mgo.Iter we can use in Next(), All() or One().
func (r *result) setCursor() error {
	if r.iter == nil {
		q, err := r.query()
		if err != nil {
			return err
		}
		r.iter = q.Iter()
	}
	return nil
}

func (r *result) And(terms ...interface{}) db.Result {
	if r.queryChunks.Conditions == nil {
		return r.Where(terms)
	}
	r.queryChunks.Conditions = map[string]interface{}{
		"$and": []interface{}{
			r.queryChunks.Conditions,
			r.c.compileQuery(terms...),
		},
	}
	return r
}

func (r *result) Where(terms ...interface{}) db.Result {
	r.queryChunks.Conditions = r.c.compileQuery(terms...)
	return r
}

// Limit determines the maximum limit of results to be returned.
func (r *result) Limit(n int) db.Result {
	r.queryChunks.Limit = n
	return r
}

// Offset determines how many documents will be skipped before starting to grab
// results.
func (r *result) Offset(n int) db.Result {
	r.queryChunks.Offset = n
	return r
}

// OrderBy determines sorting of results according to the provided names. Fields
// may be prefixed by - (minus) which means descending order, ascending order
// would be used otherwise.
func (r *result) OrderBy(fields ...interface{}) db.Result {
	ss := make([]string, len(fields))
	for i, field := range fields {
		ss[i] = fmt.Sprintf(`%v`, field)
	}
	r.queryChunks.Sort = ss
	return r
}

// String satisfies fmt.Stringer
func (r *result) String() string {
	return fmt.Sprintf("%v", r.queryChunks)
}

// Select marks the specific fields the user wants to retrieve.
func (r *result) Select(fields ...interface{}) db.Result {
	fieldslen := len(fields)
	r.queryChunks.Fields = make([]string, 0, fieldslen)
	for i := 0; i < fieldslen; i++ {
		r.queryChunks.Fields = append(r.queryChunks.Fields, fmt.Sprintf(`%v`, fields[i]))
	}
	return r
}

// All dumps all results into a pointer to an slice of structs or maps.
func (r *result) All(dst interface{}) (err error) {

	if r.c.parent.LoggingEnabled() {
		defer func(start time.Time) {
			r.c.parent.Logger().Log(&db.QueryStatus{
				Query: fmt.Sprintf("find(%s)", mustJSON(r.queryChunks.Conditions)),
				Err:   err,
				Start: start,
				End:   time.Now(),
			})
		}(time.Now())
	}

	err = r.setCursor()

	if err != nil {
		return err
	}

	err = r.iter.All(dst)

	if err != nil {
		return err
	}

	r.Close()

	return nil
}

// Group is used to group results that have the same value in the same column
// or columns.
func (r *result) Group(fields ...interface{}) db.Result {
	r.queryChunks.GroupBy = fields
	return r
}

// One fetches only one result from the resultset.
func (r *result) One(dst interface{}) (err error) {
	if r.c.parent.LoggingEnabled() {
		defer func(start time.Time) {
			r.c.parent.Logger().Log(&db.QueryStatus{
				Query: fmt.Sprintf("findOne(%s)", mustJSON(r.queryChunks.Conditions)),
				Err:   err,
				Start: start,
				End:   time.Now(),
			})
		}(time.Now())
	}

	defer r.Close()
	if !r.Next(dst) {
		return r.Err()
	}
	return nil
}

// Next fetches the next result from the resultset.
func (r *result) Next(dst interface{}) bool {
	err := r.setCursor()
	if err != nil {
		r.setErr(err)
		return false
	}
	if !r.iter.Next(dst) {
		r.setErr(err)
		return false
	}
	return true
}

// Delete remove the matching items from the collection.
func (r *result) Delete() (err error) {
	if r.c.parent.LoggingEnabled() {
		defer func(start time.Time) {
			r.c.parent.Logger().Log(&db.QueryStatus{
				Query: fmt.Sprintf("remove(%s)", mustJSON(r.queryChunks.Conditions)),
				Err:   err,
				Start: start,
				End:   time.Now(),
			})
		}(time.Now())
	}

	_, err = r.c.collection.RemoveAll(r.queryChunks.Conditions)
	if err != nil {
		return err
	}
	return nil
}

// Close closes the result set.
func (r *result) Close() error {
	var err error
	if r.iter != nil {
		err = r.iter.Close()
		r.iter = nil
	}
	return err
}

// Update modified matching items from the collection with values of the given
// map or struct.
func (r *result) Update(src interface{}) (err error) {
	updateSet := map[string]interface{}{"$set": src}

	if r.c.parent.LoggingEnabled() {
		defer func(start time.Time) {
			r.c.parent.Logger().Log(&db.QueryStatus{
				Query: fmt.Sprintf("update(%s, %s)", mustJSON(r.queryChunks.Conditions), mustJSON(updateSet)),
				Err:   err,
				Start: start,
				End:   time.Now(),
			})
		}(time.Now())
	}

	_, err = r.c.collection.UpdateAll(r.queryChunks.Conditions, updateSet)
	if err != nil {
		return err
	}
	return nil
}

// query executes a mgo query.
func (r *result) query() (*mgo.Query, error) {
	var err error

	q := r.c.collection.Find(r.queryChunks.Conditions)

	if len(r.queryChunks.GroupBy) > 0 {
		return nil, db.ErrUnsupported
	}

	if r.queryChunks.Offset > 0 {
		q = q.Skip(r.queryChunks.Offset)
	}

	if r.queryChunks.Limit > 0 {
		q = q.Limit(r.queryChunks.Limit)
	}

	if len(r.queryChunks.Fields) > 0 {
		selectedFields := bson.M{}
		for _, field := range r.queryChunks.Fields {
			if field == `*` {
				break
			}
			selectedFields[field] = true
		}
		if len(selectedFields) > 0 {
			q = q.Select(selectedFields)
		}
	}

	if len(r.queryChunks.Sort) > 0 {
		q.Sort(r.queryChunks.Sort...)
	}

	return q, err
}

// Count counts matching elements.
func (r *result) Count() (total uint64, err error) {
	if r.c.parent.LoggingEnabled() {
		defer func(start time.Time) {
			r.c.parent.Logger().Log(&db.QueryStatus{
				Query: fmt.Sprintf("find(%s).count()", mustJSON(r.queryChunks.Conditions)),
				Err:   err,
				Start: start,
				End:   time.Now(),
			})
		}(time.Now())
	}

	q := r.c.collection.Find(r.queryChunks.Conditions)
	var c int
	c, err = q.Count()
	return uint64(c), err
}

func (r *result) debugQuery(action string) string {
	query := fmt.Sprintf("db.%s.%s", r.c.collection.Name, action)

	if r.queryChunks.Limit > 0 {
		query = fmt.Sprintf("%s.limit(%d)", query, r.queryChunks.Limit)
	}
	if r.queryChunks.Offset > 0 {
		query = fmt.Sprintf("%s.offset(%d)", query, r.queryChunks.Offset)
	}
	if len(r.queryChunks.Fields) > 0 {
		selectedFields := bson.M{}
		for _, field := range r.queryChunks.Fields {
			if field == `*` {
				break
			}
			selectedFields[field] = true
		}
		if len(selectedFields) > 0 {
			query = fmt.Sprintf("%s.select(%v)", query, selectedFields)
		}
	}
	if len(r.queryChunks.GroupBy) > 0 {
		escaped := make([]string, len(r.queryChunks.GroupBy))
		for i := range r.queryChunks.GroupBy {
			escaped[i] = string(mustJSON(r.queryChunks.GroupBy[i]))
		}
		query = fmt.Sprintf("%s.groupBy(%v)", query, strings.Join(escaped, ", "))
	}
	if len(r.queryChunks.Sort) > 0 {
		escaped := make([]string, len(r.queryChunks.Sort))
		for i := range r.queryChunks.Sort {
			escaped[i] = string(mustJSON(r.queryChunks.Sort[i]))
		}
		query = fmt.Sprintf("%s.sort(%s)", query, strings.Join(escaped, ", "))
	}
	return query
}

func mustJSON(in interface{}) (out []byte) {
	out, err := json.Marshal(in)
	if err != nil {
		panic(err)
	}
	return out
}
