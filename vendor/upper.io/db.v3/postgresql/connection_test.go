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

import "testing"

func TestConnectionURL(t *testing.T) {
	c := ConnectionURL{}

	// Default connection string is empty.
	if c.String() != "" {
		t.Fatal(`Expecting default connectiong string to be empty, got:`, c.String())
	}

	// Adding a host with port.
	c.Host = "localhost:1234"

	if c.String() != "host=localhost port=1234 sslmode=disable" {
		t.Fatal(`Test failed, got:`, c.String())
	}

	// Adding a host.
	c.Host = "localhost"

	if c.String() != "host=localhost sslmode=disable" {
		t.Fatal(`Test failed, got:`, c.String())
	}

	// Adding a username.
	c.User = "Anakin"

	if c.String() != "user=Anakin host=localhost sslmode=disable" {
		t.Fatal(`Test failed, got:`, c.String())
	}

	// Adding a password with special characters.
	c.Password = "Some Sort of ' Password"

	if c.String() != `user=Anakin password=Some\ Sort\ of\ \'\ Password host=localhost sslmode=disable` {
		t.Fatal(`Test failed, got:`, c.String())
	}

	// Adding a port.
	c.Host = "localhost:1234"

	if c.String() != `user=Anakin password=Some\ Sort\ of\ \'\ Password host=localhost port=1234 sslmode=disable` {
		t.Fatal(`Test failed, got:`, c.String())
	}

	// Adding a database.
	c.Database = "MyDatabase"

	if c.String() != `user=Anakin password=Some\ Sort\ of\ \'\ Password host=localhost port=1234 dbname=MyDatabase sslmode=disable` {
		t.Fatal(`Test failed, got:`, c.String())
	}

	// Adding options.
	c.Options = map[string]string{
		"sslmode": "verify-full",
	}

	if c.String() != `user=Anakin password=Some\ Sort\ of\ \'\ Password host=localhost port=1234 dbname=MyDatabase sslmode=verify-full` {
		t.Fatal(`Test failed, got:`, c.String())
	}
}

func TestParseConnectionURL(t *testing.T) {
	var u ConnectionURL
	var s string
	var err error

	s = "postgres://anakin:skywalker@localhost/jedis"

	if u, err = ParseURL(s); err != nil {
		t.Fatal(err)
	}

	if u.User != "anakin" {
		t.Fatal("Failed to parse username.")
	}

	if u.Password != "skywalker" {
		t.Fatal("Failed to parse password.")
	}

	if u.Host != "localhost" {
		t.Fatal("Failed to parse hostname.")
	}

	if u.Database != "jedis" {
		t.Fatal("Failed to parse database.")
	}

	if u.Options["sslmode"] != "" {
		t.Fatal("Failed to parse SSLMode.")
	}

	// case with port
	s = "postgres://anakin:skywalker@localhost:1234/jedis"
	if u, err = ParseURL(s); err != nil {
		t.Fatal(err)
	}

	if u.User != "anakin" {
		t.Fatal("Failed to parse username.")
	}

	if u.Password != "skywalker" {
		t.Fatal("Failed to parse password.")
	}

	if u.Host != "localhost:1234" {
		t.Fatal("Failed to parse hostname.")
	}

	if u.Database != "jedis" {
		t.Fatal("Failed to parse database.")
	}

	if u.Options["sslmode"] != "" {
		t.Fatal("Failed to parse SSLMode.")
	}

	s = "postgres://anakin:skywalker@localhost/jedis?sslmode=verify-full"

	if u, err = ParseURL(s); err != nil {
		t.Fatal(err)
	}

	if u.Options["sslmode"] != "verify-full" {
		t.Fatal("Failed to parse SSLMode.")
	}

	s = "user=anakin password=skywalker host=localhost dbname=jedis"

	if u, err = ParseURL(s); err != nil {
		t.Fatal(err)
	}

	if u.User != "anakin" {
		t.Fatal("Failed to parse username.")
	}

	if u.Password != "skywalker" {
		t.Fatal("Failed to parse password.")
	}

	if u.Host != "localhost" {
		t.Fatal("Failed to parse hostname.")
	}

	if u.Database != "jedis" {
		t.Fatal("Failed to parse database.")
	}

	if u.Options["sslmode"] != "" {
		t.Fatal("Failed to parse SSLMode.")
	}

	s = "user=anakin password=skywalker host=localhost dbname=jedis sslmode=verify-full"

	if u, err = ParseURL(s); err != nil {
		t.Fatal(err)
	}

	if u.Options["sslmode"] != "verify-full" {
		t.Fatal("Failed to parse SSLMode.")
	}

	s = "user=anakin password=skywalker host=localhost dbname=jedis sslmode=verify-full timezone=UTC"

	if u, err = ParseURL(s); err != nil {
		t.Fatal(err)
	}

	if len(u.Options) != 2 {
		t.Fatal("Expecting exactly two options.")
	}

	if u.Options["sslmode"] != "verify-full" {
		t.Fatal("Failed to parse SSLMode.")
	}

	if u.Options["timezone"] != "UTC" {
		t.Fatal("Failed to parse timezone.")
	}
}
