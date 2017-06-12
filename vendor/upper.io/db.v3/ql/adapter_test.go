// Copyright (c) 2012-today The upper.io/db authors. All rights reserved.
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

//go:generate bash -c "sed s/ADAPTER/ql/g ../internal/sqladapter/testing/adapter.go.tpl > generated_test.go"
package ql

import (
	"database/sql"
	"os"

	"upper.io/db.v3/lib/sqlbuilder"
)

const (
	testTimeZone = "Canada/Eastern"
)

var settings = ConnectionURL{
	Database: os.Getenv("DB_NAME"),
}

func tearUp() error {
	sess := mustOpen()
	defer sess.Close()

	batch := []string{
		`DROP TABLE IF EXISTS artist`,

		`CREATE TABLE artist (
			name string
		)`,

		`DROP TABLE IF EXISTS publication`,

		`CREATE TABLE publication (
			title string,
			author_id int
		)`,

		`DROP TABLE IF EXISTS review`,

		`CREATE TABLE review (
			publication_id int,
			name string,
			comments string,
			created time
		)`,

		`DROP TABLE IF EXISTS data_types`,

		`CREATE TABLE data_types (
			_uint uint,
			_uint8 uint8,
			_uint16 uint16,
			_uint32 uint32,
			_uint64 uint64,
			_int int,
			_int8 int8,
			_int16 int16,
			_int32 int32,
			_int64 int64,
			_float32 float32,
			_float64 float64,
			_bool bool,
			_string string,
			_date time,
			_nildate time,
			_ptrdate time,
			_defaultdate time,
			_time time
		)`,

		`DROP TABLE IF EXISTS stats_test`,

		`CREATE TABLE stats_test (
			id uint,
			numeric int64,
			value int64
		)`,

		`DROP TABLE IF EXISTS composite_keys`,

		`-- Composite keys are currently not supported in QL.
		CREATE TABLE composite_keys (
		-- code string,
		-- user_id string,
			some_val string,
		-- primary key (code, user_id)
		)`,
	}

	driver := sess.Driver().(*sql.DB)
	tx, err := driver.Begin()
	if err != nil {
		return err
	}

	for _, s := range batch {
		if _, err := tx.Exec(s); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}

func cleanUpCheck(sess sqlbuilder.Database) (err error) {
	// TODO: Check the number of prepared statements.
	return nil
}
