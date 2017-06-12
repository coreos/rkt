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

	if c.String() != "mongodb://myfilename" {
		t.Fatal(`Test failed, got:`, c.String())
	}

	// Adding an option.
	c.Options = map[string]string{
		"cache": "foobar",
		"mode":  "ro",
	}

	// Adding username and password
	c.User = "user"
	c.Password = "pass"

	// Setting host.
	c.Host = "localhost"

	if c.String() != "mongodb://user:pass@localhost/myfilename?cache=foobar&mode=ro" {
		t.Fatal(`Test failed, got:`, c.String())
	}

	// Setting host and port.
	c.Host = "localhost:27017"

	if c.String() != "mongodb://user:pass@localhost:27017/myfilename?cache=foobar&mode=ro" {
		t.Fatal(`Test failed, got:`, c.String())
	}

	// Setting cluster.
	c.Host = "localhost,1.2.3.4,example.org:1234"

	if c.String() != "mongodb://user:pass@localhost,1.2.3.4,example.org:1234/myfilename?cache=foobar&mode=ro" {
		t.Fatal(`Test failed, got:`, c.String())
	}

	// Setting another database.
	c.Database = "another_database"

	if c.String() != "mongodb://user:pass@localhost,1.2.3.4,example.org:1234/another_database?cache=foobar&mode=ro" {
		t.Fatal(`Test failed, got:`, c.String())
	}

}

func TestParseConnectionURL(t *testing.T) {
	var u ConnectionURL
	var s string
	var err error

	s = "mongodb:///mydatabase"

	if u, err = ParseURL(s); err != nil {
		t.Fatal(err)
	}

	if u.Database != "mydatabase" {
		t.Fatal("Failed to parse database.")
	}

	s = "mongodb://user:pass@localhost,1.2.3.4,example.org:1234/another_database?cache=foobar&mode=ro"

	if u, err = ParseURL(s); err != nil {
		t.Fatal(err)
	}

	if u.Database != "another_database" {
		t.Fatal("Failed to get database.")
	}

	if u.Options["cache"] != "foobar" {
		t.Fatal("Expecting option.")
	}

	if u.Options["mode"] != "ro" {
		t.Fatal("Expecting option.")
	}

	if u.User != "user" {
		t.Fatal("Expecting user.")
	}

	if u.Password != "pass" {
		t.Fatal("Expecting password.")
	}

	if u.Host != "localhost,1.2.3.4,example.org:1234" {
		t.Fatal("Expecting host.")
	}

	s = "http://example.org"

	if _, err = ParseURL(s); err == nil {
		t.Fatal("Expecting error.")
	}

}
