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

package sqlite

import (
	"path/filepath"
	"testing"
)

func TestConnectionURL(t *testing.T) {

	c := ConnectionURL{}

	// Default connection string is only the protocol.
	if c.String() != "" {
		t.Fatal(`Expecting default connectiong string to be empty, got:`, c.String())
	}

	// Adding a database name.
	c.Database = "myfilename"

	absoluteName, _ := filepath.Abs(c.Database)

	if c.String() != "file://"+absoluteName+"?_busy_timeout=10000" {
		t.Fatal(`Test failed, got:`, c.String())
	}

	// Adding an option.
	c.Options = map[string]string{
		"cache": "foobar",
		"mode":  "ro",
	}

	if c.String() != "file://"+absoluteName+"?_busy_timeout=10000&cache=foobar&mode=ro" {
		t.Fatal(`Test failed, got:`, c.String())
	}

	// Setting another database.
	c.Database = "/another/database"

	if c.String() != `file:///another/database?_busy_timeout=10000&cache=foobar&mode=ro` {
		t.Fatal(`Test failed, got:`, c.String())
	}

}

func TestParseConnectionURL(t *testing.T) {
	var u ConnectionURL
	var s string
	var err error

	s = "file://mydatabase.db"

	if u, err = ParseURL(s); err != nil {
		t.Fatal(err)
	}

	if u.Database != "mydatabase.db" {
		t.Fatal("Failed to parse database.")
	}

	if u.Options["cache"] != "shared" {
		t.Fatal("If not defined, cache should be shared by default.")
	}

	s = "file:///path/to/my/database.db?_busy_timeout=10000&mode=ro&cache=foobar"

	if u, err = ParseURL(s); err != nil {
		t.Fatal(err)
	}

	if u.Database != "/path/to/my/database.db" {
		t.Fatal("Failed to parse username.")
	}

	if u.Options["cache"] != "foobar" {
		t.Fatal("Expecting option.")
	}

	if u.Options["mode"] != "ro" {
		t.Fatal("Expecting option.")
	}

	s = "http://example.org"

	if _, err = ParseURL(s); err == nil {
		t.Fatal("Expecting error.")
	}

}
