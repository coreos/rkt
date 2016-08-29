// Copyright 2016 The rkt Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kvdb

import (
	"os"

	"github.com/boltdb/bolt"
)

// Store represents a store of rendered images
type DB struct {
	dbfile string
	mode   os.FileMode
}

func NewDB(dbfile string, mode os.FileMode) *DB {
	return &DB{dbfile: dbfile, mode: mode}
}

type txfunc func(*bolt.Tx) error

func (db *DB) Do(ro bool, fns ...txfunc) error {
	bdb, err := bolt.Open(db.dbfile, db.mode, &bolt.Options{ReadOnly: ro})
	if err != nil {
		return err
	}
	defer bdb.Close()

	tx, err := bdb.Begin(!ro)
	if err != nil {
		return err
	}

	for _, fn := range fns {
		if err := fn(tx); err != nil {
			if !ro {
				tx.Rollback()
			}
			return err
		}
	}
	// Commit the transaction and check for error.
	if !ro {
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func (db *DB) DoRW(fns ...txfunc) error {
	return db.Do(false, fns...)
}

func (db *DB) DoRO(fns ...txfunc) error {
	return db.Do(true, fns...)
}
