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

//go:generate bash -c "sed s/ADAPTER/mysql/g ../internal/sqladapter/testing/adapter.go.tpl > generated_test.go"
package mysql

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	"upper.io/db.v3/internal/sqladapter"
	"upper.io/db.v3/lib/sqlbuilder"
)

const (
	testTimeZone = "Canada/Eastern"
)

var settings = ConnectionURL{
	Database: os.Getenv("DB_NAME"),
	User:     os.Getenv("DB_USERNAME"),
	Password: os.Getenv("DB_PASSWORD"),
	Host:     os.Getenv("DB_HOST") + ":" + os.Getenv("DB_PORT"),
	Options: map[string]string{
		// See https://github.com/go-sql-driver/mysql/issues/9
		"parseTime": "true",
		// Might require you to use mysql_tzinfo_to_sql /usr/share/zoneinfo | mysql -u root -p mysql
		"time_zone": fmt.Sprintf(`"%s"`, testTimeZone),
	},
}

func tearUp() error {
	sess := mustOpen()
	defer sess.Close()

	batch := []string{
		`DROP TABLE IF EXISTS artist`,

		`CREATE TABLE artist (
			id BIGINT(20) UNSIGNED NOT NULL AUTO_INCREMENT,
			PRIMARY KEY(id),
			name VARCHAR(60)
		)`,

		`DROP TABLE IF EXISTS publication`,

		`CREATE TABLE publication (
			id BIGINT(20) UNSIGNED NOT NULL AUTO_INCREMENT,
			PRIMARY KEY(id),
			title VARCHAR(80),
			author_id BIGINT(20)
		)`,

		`DROP TABLE IF EXISTS review`,

		`CREATE TABLE review (
			id BIGINT(20) UNSIGNED NOT NULL AUTO_INCREMENT,
			PRIMARY KEY(id),
			publication_id BIGINT(20),
			name VARCHAR(80),
			comments TEXT,
			created DATETIME NOT NULL
		)`,

		`DROP TABLE IF EXISTS data_types`,

		`CREATE TABLE data_types (
			id BIGINT(20) UNSIGNED NOT NULL AUTO_INCREMENT,
			PRIMARY KEY(id),
			_uint INT(10) UNSIGNED DEFAULT 0,
			_uint8 INT(10) UNSIGNED DEFAULT 0,
			_uint16 INT(10) UNSIGNED DEFAULT 0,
			_uint32 INT(10) UNSIGNED DEFAULT 0,
			_uint64 INT(10) UNSIGNED DEFAULT 0,
			_int INT(10) DEFAULT 0,
			_int8 INT(10) DEFAULT 0,
			_int16 INT(10) DEFAULT 0,
			_int32 INT(10) DEFAULT 0,
			_int64 INT(10) DEFAULT 0,
			_float32 DECIMAL(10,6),
			_float64 DECIMAL(10,6),
			_bool TINYINT(1),
			_string text,
			_date TIMESTAMP NULL,
			_nildate DATETIME NULL,
			_ptrdate DATETIME NULL,
			_defaultdate TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			_time BIGINT UNSIGNED NOT NULL DEFAULT 0
		)`,

		`DROP TABLE IF EXISTS stats_test`,

		`CREATE TABLE stats_test (
			id BIGINT(20) UNSIGNED NOT NULL AUTO_INCREMENT, PRIMARY KEY(id),
			` + "`numeric`" + ` INT(10),
			` + "`value`" + ` INT(10)
		)`,

		`DROP TABLE IF EXISTS composite_keys`,

		`CREATE TABLE composite_keys (
			code VARCHAR(255) default '',
			user_id VARCHAR(255) default '',
			some_val VARCHAR(255) default '',
			primary key (code, user_id)
		)`,
	}

	for _, s := range batch {
		driver := sess.Driver().(*sql.DB)
		if _, err := driver.Exec(s); err != nil {
			return err
		}
	}

	return nil
}

func getStats(sess sqlbuilder.Database) (map[string]int, error) {
	stats := make(map[string]int)

	res, err := sess.Driver().(*sql.DB).Query(`SHOW GLOBAL STATUS LIKE '%stmt%'`)
	if err != nil {
		return nil, err
	}
	var result struct {
		VariableName string `db:"Variable_name"`
		Value        int    `db:"Value"`
	}

	iter := sqlbuilder.NewIterator(res)
	for iter.Next(&result) {
		stats[result.VariableName] = result.Value
	}

	return stats, nil
}

func cleanUpCheck(sess sqlbuilder.Database) (err error) {
	var stats map[string]int

	stats, err = getStats(sess)
	if err != nil {
		return err
	}

	if activeStatements := sqladapter.NumActiveStatements(); activeStatements > 128 {
		return fmt.Errorf("Expecting active statements to be at most 128, got %d", activeStatements)
	}

	sess.ClearCache()

	if activeStatements := sqladapter.NumActiveStatements(); activeStatements != 0 {
		return fmt.Errorf("Expecting active statements to be 0, got %d", activeStatements)
	}

	for i := 0; i < 10; i++ {
		stats, err = getStats(sess)
		if err != nil {
			return err
		}

		if stats["Prepared_stmt_count"] != 0 {
			time.Sleep(time.Millisecond * 200) // Sometimes it takes a bit to clean prepared statements
			err = fmt.Errorf(`Expecting "Prepared_stmt_count" to be 0, got %d`, stats["Prepared_stmt_count"])
			continue
		}
		break
	}

	return err
}
