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

//go:generate bash -c "sed s/ADAPTER/sqlite/g ../internal/sqladapter/testing/adapter.go.tpl > generated_test.go"
package sqlite

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
		`PRAGMA foreign_keys=OFF`,

		`BEGIN TRANSACTION`,

		`DROP TABLE IF EXISTS artist`,

		`CREATE TABLE artist (
			id integer primary key,
			name varchar(60)
		)`,

		`DROP TABLE IF EXISTS publication`,

		`CREATE TABLE publication (
			id integer primary key,
			title varchar(80),
			author_id integer
		)`,

		`DROP TABLE IF EXISTS review`,

		`CREATE TABLE review (
			id integer primary key,
			publication_id integer,
			name varchar(80),
			comments text,
			created datetime
		)`,

		`DROP TABLE IF EXISTS data_types`,

		`CREATE TABLE data_types (
			id integer primary key,
		 _uint integer,
		 _uintptr integer,
		 _uint8 integer,
		 _uint16 int,
		 _uint32 int,
		 _uint64 int,
		 _int integer,
		 _int8 integer,
		 _int16 integer,
		 _int32 integer,
		 _int64 integer,
		 _float32 real,
		 _float64 real,
		 _byte integer,
		 _rune integer,
		 _bool integer,
		 _string text,
		 _date datetime,
		 _nildate datetime,
		 _ptrdate datetime,
		 _defaultdate datetime default current_timestamp,
		 _time text
		)`,

		`DROP TABLE IF EXISTS stats_test`,

		`CREATE TABLE stats_test (
			id integer primary key,
			numeric integer,
			value integer
		)`,

		`DROP TABLE IF EXISTS composite_keys`,

		`CREATE TABLE composite_keys (
			code VARCHAR(255) default '',
			user_id VARCHAR(255) default '',
			some_val VARCHAR(255) default '',
			primary key (code, user_id)
		)`,

		`COMMIT`,
	}

	for _, s := range batch {
		driver := sess.Driver().(*sql.DB)
		if _, err := driver.Exec(s); err != nil {
			return err
		}
	}

	return nil
}

func cleanUpCheck(sess sqlbuilder.Database) (err error) {
	// TODO: Check the number of prepared statements.
	return nil
}
