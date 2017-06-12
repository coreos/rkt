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
	"fmt"
	"strings"
	"sync"

	"reflect"

	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"upper.io/db.v3"
)

// Collection represents a mongodb collection.
type Collection struct {
	name       string
	parent     *Source
	collection *mgo.Collection
}

type chunks struct {
	Fields     []string
	Limit      int
	Offset     int
	Sort       []string
	Conditions interface{}
	GroupBy    []interface{}
}

var (
	// idCache should be a struct if we're going to cache more than just
	// _id field here
	idCache      = make(map[reflect.Type]string, 0)
	idCacheMutex sync.RWMutex
)

// Find creates a result set with the given conditions.
func (col *Collection) Find(terms ...interface{}) db.Result {
	queryChunks := &chunks{}

	// No specific fields given.
	if len(queryChunks.Fields) == 0 {
		queryChunks.Fields = []string{"*"}
	}

	queryChunks.Conditions = col.compileQuery(terms...)

	// Actually executing query.
	r := &result{
		c:           col,
		queryChunks: queryChunks,
	}

	return r
}

// compileStatement transforms conditions into something *mgo.Session can
// understand.
func compileStatement(cond db.Cond) bson.M {
	conds := bson.M{}

	// Walking over conditions
	for fieldI, value := range cond {
		// Removing leading or trailing spaces.
		field := strings.TrimSpace(fmt.Sprintf("%v", fieldI))

		chunks := strings.SplitN(field, ` `, 2)

		var op string

		if len(chunks) > 1 {
			switch chunks[1] {
			case `IN`:
				op = `$in`
			case `NOT IN`:
				op = `$nin`
			case `>`:
				op = `$gt`
			case `<`:
				op = `$lt`
			case `<=`:
				op = `$lte`
			case `>=`:
				op = `$gte`
			default:
				op = chunks[1]
			}
		}

		if op == "" {
			conds[chunks[0]] = value
		} else {
			conds[chunks[0]] = bson.M{op: value}
		}

	}

	return conds
}

// compileConditions compiles terms into something *mgo.Session can
// understand.
func (col *Collection) compileConditions(term interface{}) interface{} {

	switch t := term.(type) {
	case []interface{}:
		values := []interface{}{}
		for i := range t {
			value := col.compileConditions(t[i])
			if value != nil {
				values = append(values, value)
			}
		}
		if len(values) > 0 {
			return values
		}
	case db.Cond:
		return compileStatement(t)
	case db.Compound:
		values := []interface{}{}

		for _, s := range t.Sentences() {
			values = append(values, col.compileConditions(s))
		}

		var op string
		switch t.Operator() {
		case db.OperatorOr:
			op = `$or`
		default:
			op = `$and`
		}

		return bson.M{op: values}
	}
	return nil
}

// compileQuery compiles terms into something that *mgo.Session can
// understand.
func (col *Collection) compileQuery(terms ...interface{}) interface{} {
	var query interface{}

	compiled := col.compileConditions(terms)

	if compiled != nil {
		conditions := compiled.([]interface{})
		if len(conditions) == 1 {
			query = conditions[0]
		} else {
			// this should be correct.
			// query = map[string]interface{}{"$and": conditions}

			// attempt to workaround https://jira.mongodb.org/browse/SERVER-4572
			mapped := map[string]interface{}{}
			for _, v := range conditions {
				for kk := range v.(map[string]interface{}) {
					mapped[kk] = v.(map[string]interface{})[kk]
				}
			}

			query = mapped
		}
	} else {
		query = map[string]interface{}{}
	}

	return query
}

// Name returns the name of the table or tables that form the collection.
func (col *Collection) Name() string {
	return col.collection.Name
}

// Truncate deletes all rows from the table.
func (col *Collection) Truncate() error {
	err := col.collection.DropCollection()

	if err != nil {
		return err
	}

	return nil
}

func (col *Collection) InsertReturning(item interface{}) error {
	return db.ErrUnsupported
}

func (col *Collection) UpdateReturning(item interface{}) error {
	return db.ErrUnsupported
}

// Insert inserts an item (map or struct) into the collection.
func (col *Collection) Insert(item interface{}) (interface{}, error) {
	var err error

	id := getID(item)

	if col.parent.versionAtLeast(2, 6, 0, 0) {
		// this breaks MongoDb older than 2.6
		if _, err = col.collection.Upsert(bson.M{"_id": id}, item); err != nil {
			return nil, err
		}
	} else {
		// Allocating a new ID.
		if err = col.collection.Insert(bson.M{"_id": id}); err != nil {
			return nil, err
		}

		// Now append data the user wants to append.
		if err = col.collection.Update(bson.M{"_id": id}, item); err != nil {
			// Cleanup allocated ID
			col.collection.Remove(bson.M{"_id": id})
			return nil, err
		}
	}

	return id, nil
}

// Exists returns true if the collection exists.
func (col *Collection) Exists() bool {
	query := col.parent.database.C(`system.namespaces`).Find(map[string]string{`name`: fmt.Sprintf(`%s.%s`, col.parent.database.Name, col.collection.Name)})
	count, _ := query.Count()
	if count > 0 {
		return true
	}
	return false
}

// Fetches object _id or generates a new one if object doesn't have one or the one it has is invalid
func getID(item interface{}) interface{} {
	v := reflect.ValueOf(item) // convert interface to Value
	v = reflect.Indirect(v)    // convert pointers

	switch v.Kind() {
	case reflect.Map:
		if inItem, ok := item.(map[string]interface{}); ok {
			if id, ok := inItem["_id"]; ok {
				bsonID, ok := id.(bson.ObjectId)
				if ok {
					return bsonID
				}
			}
		}
	case reflect.Struct:
		t := v.Type()

		idCacheMutex.RLock()
		fieldName, found := idCache[t]
		idCacheMutex.RUnlock()

		if !found {
			for n := 0; n < t.NumField(); n++ {
				field := t.Field(n)
				if field.PkgPath != "" {
					continue // Private field
				}

				tag := field.Tag.Get("bson")
				if tag == "" {
					tag = field.Tag.Get("db")
				}

				if tag == "" {
					continue
				}

				parts := strings.Split(tag, ",")

				if parts[0] == "_id" {
					fieldName = field.Name
					idCacheMutex.RLock()
					idCache[t] = fieldName
					idCacheMutex.RUnlock()
					break
				}
			}
		}
		if fieldName != "" {
			if bsonID, ok := v.FieldByName(fieldName).Interface().(bson.ObjectId); ok {
				if bsonID.Valid() {
					return bsonID
				}
			} else {
				return v.FieldByName(fieldName).Interface()
			}
		}
	}

	return bson.NewObjectId()
}
