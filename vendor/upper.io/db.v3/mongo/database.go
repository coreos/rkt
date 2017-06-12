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

// Package mongo wraps the gopkg.in/mgo.v2 MongoDB driver. See
// https://upper.io/db.v3/mongo for documentation, particularities and usage
// examples.
package mongo // import "upper.io/db.v3/mongo"

import (
	"strings"
	"sync"
	"time"

	"gopkg.in/mgo.v2"
	"upper.io/db.v3"
)

// Adapter holds the name of the mongodb adapter.
const Adapter = `mongo`

var connTimeout = time.Second * 5

// Source represents a MongoDB database.
type Source struct {
	db.Settings

	name          string
	connURL       db.ConnectionURL
	session       *mgo.Session
	database      *mgo.Database
	version       []int
	collections   map[string]*Collection
	collectionsMu sync.Mutex
}

func init() {
	db.RegisterAdapter(Adapter, &db.AdapterFuncMap{
		Open: Open,
	})
}

// Open stablishes a new connection to a SQL server.
func Open(settings db.ConnectionURL) (db.Database, error) {
	d := &Source{Settings: db.NewSettings()}
	if err := d.Open(settings); err != nil {
		return nil, err
	}
	return d, nil
}

func (s *Source) ConnectionURL() db.ConnectionURL {
	return s.connURL
}

// SetConnMaxLifetime is not supported.
func (s *Source) SetConnMaxLifetime(time.Duration) {
	s.Settings.SetConnMaxLifetime(time.Duration(0))
}

// SetMaxIdleConns is not supported.
func (s *Source) SetMaxIdleConns(int) {
	s.Settings.SetMaxIdleConns(0)
}

// SetMaxOpenConns is not supported.
func (s *Source) SetMaxOpenConns(int) {
	s.Settings.SetMaxOpenConns(0)
}

// Name returns the name of the database.
func (s *Source) Name() string {
	return s.name
}

// Open attempts to connect to the database.
func (s *Source) Open(connURL db.ConnectionURL) error {
	s.connURL = connURL
	return s.open()
}

// Clone returns a cloned db.Database session.
func (s *Source) Clone() (db.Database, error) {
	newSession := s.session.Copy()
	clone := &Source{
		Settings: db.NewSettings(),

		name:        s.name,
		connURL:     s.connURL,
		session:     newSession,
		database:    newSession.DB(s.database.Name),
		version:     s.version,
		collections: map[string]*Collection{},
	}
	return clone, nil
}

// NewTransaction should support transactions, but it doesn't as MongoDB
// currently does not support them.
func (s *Source) NewTransaction() (db.Tx, error) {
	return nil, db.ErrUnsupported
}

// Ping checks whether a connection to the database is still alive by pinging
// it, establishing a connection if necessary.
func (s *Source) Ping() error {
	return s.session.Ping()
}

func (s *Source) ClearCache() {
	s.collectionsMu.Lock()
	defer s.collectionsMu.Unlock()
	s.collections = make(map[string]*Collection)
}

// Driver returns the underlying *mgo.Session instance.
func (s *Source) Driver() interface{} {
	return s.session
}

func (s *Source) open() error {
	var err error

	if s.session, err = mgo.DialWithTimeout(s.connURL.String(), connTimeout); err != nil {
		return err
	}

	s.collections = map[string]*Collection{}
	s.database = s.session.DB("")

	return nil
}

// Close terminates the current database session.
func (s *Source) Close() error {
	if s.session != nil {
		s.session.Close()
	}
	return nil
}

// Collections returns a list of non-system tables from the database.
func (s *Source) Collections() (cols []string, err error) {
	var rawcols []string
	var col string

	if rawcols, err = s.database.CollectionNames(); err != nil {
		return nil, err
	}

	cols = make([]string, 0, len(rawcols))

	for _, col = range rawcols {
		if !strings.HasPrefix(col, "system.") {
			cols = append(cols, col)
		}
	}

	return cols, nil
}

// Collection returns a collection by name.
func (s *Source) Collection(name string) db.Collection {
	s.collectionsMu.Lock()
	defer s.collectionsMu.Unlock()

	var col *Collection
	var ok bool

	if col, ok = s.collections[name]; !ok {
		col = &Collection{
			parent:     s,
			collection: s.database.C(name),
		}
		s.collections[name] = col
	}

	return col
}

func (s *Source) versionAtLeast(version ...int) bool {
	// only fetch this once - it makes a db call
	if len(s.version) == 0 {
		buildInfo, err := s.database.Session.BuildInfo()
		if err != nil {
			return false
		}
		s.version = buildInfo.VersionArray
	}

	// Check major version first
	if s.version[0] > version[0] {
		return true
	}

	for i := range version {
		if i == len(s.version) {
			return false
		}
		if s.version[i] < version[i] {
			return false
		}
	}
	return true
}
