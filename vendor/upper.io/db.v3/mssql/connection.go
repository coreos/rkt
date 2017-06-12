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

package mssql

import (
	"errors"
	"net/url"
)

// ConnectionURL implements a MSSQL connection struct.
type ConnectionURL struct {
	User     string
	Password string
	Database string
	Host     string
	Socket   string
	Options  map[string]string
}

func (c ConnectionURL) String() (s string) {
	if c.Host == "" && c.Database == "" && c.User == "" && c.Password == "" {
		return ""
	}

	if c.Host == "" {
		c.Host = "127.0.0.1"
	}

	if c.Database == "" {
		c.Database = "master"
	}

	params := url.Values{}
	for k, v := range c.Options {
		if k == "instance" {
			continue
		}
		params.Add(k, v)
	}
	params.Set("database", c.Database)

	u := url.URL{
		Scheme:   "sqlserver",
		Host:     c.Host,
		RawQuery: params.Encode(),
	}

	u.Path, _ = c.Options["instance"]

	if c.User != "" || c.Password != "" {
		u.User = url.UserPassword(c.User, c.Password)
	}

	return u.String()
}

// ParseURL parses s into a ConnectionURL struct.
func ParseURL(s string) (conn ConnectionURL, err error) {
	var u *url.URL

	u, err = url.Parse(s)
	if err != nil {
		return
	}

	if u.Scheme != "sqlserver" && u.Scheme != "mssql" {
		return conn, errors.New(`Expecting "sqlserver" or "mssql" schema`)
	}

	if u.Host == "" {
		conn.Host = "127.0.0.1"
	} else {
		conn.Host = u.Host
	}

	if u.User != nil {
		conn.User = u.User.Username()
		conn.Password, _ = u.User.Password()
	}

	q := u.Query()
	for k := range q {
		if k == "database" {
			conn.Database = q.Get(k)
			continue
		}
		if conn.Options == nil {
			conn.Options = make(map[string]string)
		}
		conn.Options[k] = q.Get(k)
	}

	return
}
