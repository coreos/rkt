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

package postgresql

import (
	"database/sql"

	"upper.io/db.v3"
	"upper.io/db.v3/internal/sqladapter"
)

// collection is the actual implementation of a collection.
type collection struct {
	sqladapter.BaseCollection // Leveraged by sqladapter

	d    *database
	name string
}

var (
	_ = sqladapter.Collection(&collection{})
	_ = db.Collection(&collection{})
)

// newCollection binds *collection with sqladapter.
func newCollection(d *database, name string) *collection {
	c := &collection{
		name: name,
		d:    d,
	}
	c.BaseCollection = sqladapter.NewBaseCollection(c)
	return c
}

func (c *collection) Name() string {
	return c.name
}

func (c *collection) Database() sqladapter.Database {
	return c.d
}

// Insert inserts an item (map or struct) into the collection.
func (c *collection) Insert(item interface{}) (interface{}, error) {
	var err error

	pKey := c.BaseCollection.PrimaryKeys()

	q := c.d.InsertInto(c.Name()).Values(item)

	if len(pKey) == 0 {
		// There is no primary key.
		var res sql.Result

		if res, err = q.Exec(); err != nil {
			return nil, err
		}

		// Attempt to use LastInsertId() (probably won't work, but the Exec()
		// succeeded, so we can safely ignore the error from LastInsertId()).
		lastID, _ := res.LastInsertId()

		return lastID, nil
	}

	// Asking the database to return the primary key after insertion.
	q = q.Returning(pKey...)

	var keyMap db.Cond
	if err = q.Iterator().One(&keyMap); err != nil {
		return nil, err
	}

	// The IDSetter interface does not match, look for another interface match.
	if len(keyMap) == 1 {
		return keyMap[pKey[0]], nil
	}

	// This was a compound key and no interface matched it, let's return a map.
	return keyMap, nil
}
