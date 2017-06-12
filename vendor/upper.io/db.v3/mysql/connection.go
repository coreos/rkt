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
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
)

// From https://github.com/go-sql-driver/mysql/blob/master/utils.go
var (
	errInvalidDSNUnescaped = errors.New("Invalid DSN: Did you forget to escape a param value?")
	errInvalidDSNAddr      = errors.New("Invalid DSN: Network Address not terminated (missing closing brace)")
	errInvalidDSNNoSlash   = errors.New("Invalid DSN: Missing the slash separating the database name")
)

// From https://github.com/go-sql-driver/mysql/blob/master/utils.go
type config struct {
	user   string
	passwd string
	net    string
	addr   string
	dbname string
	params map[string]string
}

// ConnectionURL implements a MySQL connection struct.
type ConnectionURL struct {
	User     string
	Password string
	Database string
	Host     string
	Socket   string
	Options  map[string]string
}

func (c ConnectionURL) String() (s string) {

	if c.Database == "" {
		return ""
	}

	// Adding username.
	if c.User != "" {
		s = s + c.User
		// Adding password.
		if c.Password != "" {
			s = s + ":" + c.Password
		}
		s = s + "@"
	}

	// Adding protocol and address
	if c.Socket != "" {
		s = s + fmt.Sprintf("unix(%s)", c.Socket)
	} else if c.Host != "" {
		host, port, err := net.SplitHostPort(c.Host)
		if err != nil {
			host = c.Host
			port = "3306"
		}
		s = s + fmt.Sprintf("tcp(%s:%s)", host, port)
	}

	// Adding database
	s = s + "/" + c.Database

	// Do we have any options?
	if c.Options == nil {
		c.Options = map[string]string{}
	}

	// Default options.
	if _, ok := c.Options["charset"]; !ok {
		c.Options["charset"] = "utf8"
	}

	if _, ok := c.Options["parseTime"]; !ok {
		c.Options["parseTime"] = "true"
	}

	// Converting options into URL values.
	vv := url.Values{}

	for k, v := range c.Options {
		vv.Set(k, v)
	}

	// Inserting options.
	if p := vv.Encode(); p != "" {
		s = s + "?" + p
	}

	return s
}

// ParseURL parses s into a ConnectionURL struct.
func ParseURL(s string) (conn ConnectionURL, err error) {
	var cfg *config

	if cfg, err = parseDSN(s); err != nil {
		return
	}

	conn.User = cfg.user
	conn.Password = cfg.passwd

	if cfg.net == "unix" {
		conn.Socket = cfg.addr
	} else if cfg.net == "tcp" {
		conn.Host = cfg.addr
	}

	conn.Database = cfg.dbname

	conn.Options = map[string]string{}

	for k, v := range cfg.params {
		conn.Options[k] = v
	}

	return
}

// from https://github.com/go-sql-driver/mysql/blob/master/utils.go
// parseDSN parses the DSN string to a config
func parseDSN(dsn string) (cfg *config, err error) {
	// New config with some default values
	cfg = &config{}

	// TODO: use strings.IndexByte when we can depend on Go 1.2

	// [user[:password]@][net[(addr)]]/dbname[?param1=value1&paramN=valueN]
	// Find the last '/' (since the password or the net addr might contain a '/')
	foundSlash := false
	for i := len(dsn) - 1; i >= 0; i-- {
		if dsn[i] == '/' {
			foundSlash = true
			var j, k int

			// left part is empty if i <= 0
			if i > 0 {
				// [username[:password]@][protocol[(address)]]
				// Find the last '@' in dsn[:i]
				for j = i; j >= 0; j-- {
					if dsn[j] == '@' {
						// username[:password]
						// Find the first ':' in dsn[:j]
						for k = 0; k < j; k++ {
							if dsn[k] == ':' {
								cfg.passwd = dsn[k+1 : j]
								break
							}
						}
						cfg.user = dsn[:k]

						break
					}
				}

				// [protocol[(address)]]
				// Find the first '(' in dsn[j+1:i]
				for k = j + 1; k < i; k++ {
					if dsn[k] == '(' {
						// dsn[i-1] must be == ')' if an address is specified
						if dsn[i-1] != ')' {
							if strings.ContainsRune(dsn[k+1:i], ')') {
								return nil, errInvalidDSNUnescaped
							}
							return nil, errInvalidDSNAddr
						}
						cfg.addr = dsn[k+1 : i-1]
						break
					}
				}
				cfg.net = dsn[j+1 : k]
			}

			// dbname[?param1=value1&...&paramN=valueN]
			// Find the first '?' in dsn[i+1:]
			for j = i + 1; j < len(dsn); j++ {
				if dsn[j] == '?' {
					if err = parseDSNParams(cfg, dsn[j+1:]); err != nil {
						return
					}
					break
				}
			}
			cfg.dbname = dsn[i+1 : j]

			break
		}
	}

	if !foundSlash && len(dsn) > 0 {
		return nil, errInvalidDSNNoSlash
	}

	// Set default network if empty
	if cfg.net == "" {
		cfg.net = "tcp"
	}

	// Set default address if empty
	if cfg.addr == "" {
		switch cfg.net {
		case "tcp":
			cfg.addr = "127.0.0.1:3306"
		case "unix":
			cfg.addr = "/tmp/mysql.sock"
		default:
			return nil, errors.New("Default addr for network '" + cfg.net + "' unknown")
		}

	}

	return
}

// From https://github.com/go-sql-driver/mysql/blob/master/utils.go
// parseDSNParams parses the DSN "query string"
// Values must be url.QueryEscape'ed
func parseDSNParams(cfg *config, params string) (err error) {
	for _, v := range strings.Split(params, "&") {
		param := strings.SplitN(v, "=", 2)
		if len(param) != 2 {
			continue
		}

		value := param[1]

		// lazy init
		if cfg.params == nil {
			cfg.params = make(map[string]string)
		}

		if cfg.params[param[0]], err = url.QueryUnescape(value); err != nil {
			return
		}
	}

	return
}
