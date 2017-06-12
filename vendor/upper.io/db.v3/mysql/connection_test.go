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

package mysql

import (
	"testing"
)

func TestConnectionURL(t *testing.T) {

	c := ConnectionURL{}

	// Zero value equals to an empty string.
	if c.String() != "" {
		t.Fatal(`Expecting default connectiong string to be empty, got:`, c.String())
	}

	// Adding a database name.
	c.Database = "mydbname"

	if c.String() != "/mydbname?charset=utf8&parseTime=true" {
		t.Fatal(`Test failed, got:`, c.String())
	}

	// Adding an option.
	c.Options = map[string]string{
		"charset": "utf8mb4,utf8",
		"sys_var": "esc@ped",
	}

	if c.String() != "/mydbname?charset=utf8mb4%2Cutf8&parseTime=true&sys_var=esc%40ped" {
		t.Fatal(`Test failed, got:`, c.String())
	}

	// Setting default options
	c.Options = nil

	// Setting user and password.
	c.User = "user"
	c.Password = "pass"

	if c.String() != `user:pass@/mydbname?charset=utf8&parseTime=true` {
		t.Fatal(`Test failed, got:`, c.String())
	}

	// Setting host.
	c.Host = "1.2.3.4:3306"

	if c.String() != `user:pass@tcp(1.2.3.4:3306)/mydbname?charset=utf8&parseTime=true` {
		t.Fatal(`Test failed, got:`, c.String())
	}

	// Setting socket.
	c.Socket = "/path/to/socket"

	if c.String() != `user:pass@unix(/path/to/socket)/mydbname?charset=utf8&parseTime=true` {
		t.Fatal(`Test failed, got:`, c.String())
	}

}

func TestParseConnectionURL(t *testing.T) {
	var u ConnectionURL
	var s string
	var err error

	s = "user:pass@unix(/path/to/socket)/mydbname?charset=utf8"

	if u, err = ParseURL(s); err != nil {
		t.Fatal(err)
	}

	if u.User != "user" {
		t.Fatal("Expecting username.")
	}

	if u.Password != "pass" {
		t.Fatal("Expecting password.")
	}

	if u.Socket != "/path/to/socket" {
		t.Fatal("Expecting socket.")
	}

	if u.Database != "mydbname" {
		t.Fatal("Expecting database.")
	}

	if u.Options["charset"] != "utf8" {
		t.Fatal("Expecting charset.")
	}

	s = "user:pass@tcp(1.2.3.4:5678)/mydbname?charset=utf8"

	if u, err = ParseURL(s); err != nil {
		t.Fatal(err)
	}

	if u.User != "user" {
		t.Fatal("Expecting username.")
	}

	if u.Password != "pass" {
		t.Fatal("Expecting password.")
	}

	if u.Host != "1.2.3.4:5678" {
		t.Fatal("Expecting host.")
	}

	if u.Database != "mydbname" {
		t.Fatal("Expecting database.")
	}

	if u.Options["charset"] != "utf8" {
		t.Fatal("Expecting charset.")
	}

}
