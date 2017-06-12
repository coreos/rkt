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
	"fmt"
	"net/url"
	"strings"
)

const connectionScheme = `mongodb`

// ConnectionURL implements a MongoDB connection struct.
type ConnectionURL struct {
	User     string
	Password string
	Host     string
	Database string
	Options  map[string]string
}

func (c ConnectionURL) String() (s string) {

	if c.Database == "" {
		return ""
	}

	vv := url.Values{}

	// Do we have any options?
	if c.Options == nil {
		c.Options = map[string]string{}
	}

	// Converting options into URL values.
	for k, v := range c.Options {
		vv.Set(k, v)
	}

	// Has user?
	var userInfo *url.Userinfo

	if c.User != "" {
		if c.Password == "" {
			userInfo = url.User(c.User)
		} else {
			userInfo = url.UserPassword(c.User, c.Password)
		}
	}

	// Building URL.
	u := url.URL{
		Scheme:   connectionScheme,
		Path:     c.Database,
		Host:     c.Host,
		User:     userInfo,
		RawQuery: vv.Encode(),
	}

	return u.String()
}

// ParseURL parses s into a ConnectionURL struct.
func ParseURL(s string) (conn ConnectionURL, err error) {
	var u *url.URL

	if strings.HasPrefix(s, connectionScheme+"://") == false {
		return conn, fmt.Errorf(`Expecting mongodb:// connection scheme.`)
	}

	if u, err = url.Parse(s); err != nil {
		return conn, err
	}

	conn.Host = u.Host

	// Deleting / from start of the string.
	conn.Database = strings.Trim(u.Path, "/")

	// Adding user / password.
	if u.User != nil {
		conn.User = u.User.Username()
		conn.Password, _ = u.User.Password()
	}

	// Adding options.
	conn.Options = map[string]string{}

	var vv url.Values

	if vv, err = url.ParseQuery(u.RawQuery); err != nil {
		return conn, err
	}

	for k := range vv {
		conn.Options[k] = vv.Get(k)
	}

	return conn, err
}
