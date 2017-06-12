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

package db

import (
	"fmt"
	"sync"
)

var (
	adapters   map[string]*AdapterFuncMap
	adaptersMu sync.RWMutex
)

func init() {
	adapters = make(map[string]*AdapterFuncMap)
}

// AdapterFuncMap defines functions that need to be implemented by adapters.
type AdapterFuncMap struct {
	Open func(settings ConnectionURL) (Database, error)
}

// RegisterAdapter registers a generic Database adapter. This function must be
// called from adapter packages upon initialization.
func RegisterAdapter(name string, adapter *AdapterFuncMap) {
	adaptersMu.Lock()
	defer adaptersMu.Unlock()

	if name == "" {
		panic(`Missing adapter name`)
	}
	if _, ok := adapters[name]; ok {
		panic(`db.RegisterAdapter() called twice for adapter: ` + name)
	}
	adapters[name] = adapter
}

func adapter(name string) AdapterFuncMap {
	adaptersMu.RLock()
	defer adaptersMu.RUnlock()

	if fn, ok := adapters[name]; ok {
		return *fn
	}
	return missingAdapter(name)
}

func missingAdapter(name string) AdapterFuncMap {
	err := fmt.Errorf("upper: Missing adapter %q, forgot to import?", name)
	return AdapterFuncMap{
		Open: func(ConnectionURL) (Database, error) {
			return nil, err
		},
	}
}

// Open attempts to open a database. Returns a generic Database instance on
// success.
func Open(adapterName string, settings ConnectionURL) (Database, error) {
	return adapter(adapterName).Open(settings)
}
