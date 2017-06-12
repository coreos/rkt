// Copyright (c) 2014-present José Carlos Nieto, https://menteslibres.net/xiam
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

package cache

import (
	"container/list"
	"errors"
	"fmt"
	"strconv"
	"sync"

	"upper.io/db.v3/internal/cache/hashstructure"
)

const defaultCapacity = 128

// Cache holds a map of volatile key -> values.
type Cache struct {
	cache    map[string]*list.Element
	li       *list.List
	capacity int
	mu       sync.RWMutex
}

type item struct {
	key   string
	value interface{}
}

// NewCacheWithCapacity initializes a new caching space with the given
// capacity.
func NewCacheWithCapacity(capacity int) (*Cache, error) {
	if capacity < 1 {
		return nil, errors.New("Capacity must be greater than zero.")
	}
	return &Cache{
		cache:    make(map[string]*list.Element),
		li:       list.New(),
		capacity: capacity,
	}, nil
}

// NewCache initializes a new caching space with default settings.
func NewCache() *Cache {
	c, err := NewCacheWithCapacity(defaultCapacity)
	if err != nil {
		panic(err.Error()) // Should never happen as we're not providing a negative defaultCapacity.
	}
	return c
}

// Read attempts to retrieve a cached value as a string, if the value does not
// exists returns an empty string and false.
func (c *Cache) Read(h Hashable) (string, bool) {
	if v, ok := c.ReadRaw(h); ok {
		if s, ok := v.(string); ok {
			return s, true
		}
	}
	return "", false
}

// ReadRaw attempts to retrieve a cached value as an interface{}, if the value
// does not exists returns nil and false.
func (c *Cache) ReadRaw(h Hashable) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	data, ok := c.cache[h.Hash()]
	if ok {
		return data.Value.(*item).value, true
	}
	return nil, false
}

// Write stores a value in memory. If the value already exists its overwritten.
func (c *Cache) Write(h Hashable, value interface{}) {
	key := h.Hash()

	c.mu.Lock()
	defer c.mu.Unlock()

	if el, ok := c.cache[key]; ok {
		el.Value.(*item).value = value
		c.li.MoveToFront(el)
		return
	}

	c.cache[key] = c.li.PushFront(&item{key, value})

	for c.li.Len() > c.capacity {
		el := c.li.Remove(c.li.Back())
		delete(c.cache, el.(*item).key)
		if p, ok := el.(*item).value.(HasOnPurge); ok {
			p.OnPurge()
		}
	}
}

// Clear generates a new memory space, leaving the old memory unreferenced, so
// it can be claimed by the garbage collector.
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, el := range c.cache {
		if p, ok := el.Value.(*item).value.(HasOnPurge); ok {
			p.OnPurge()
		}
	}
	c.cache = make(map[string]*list.Element)
	c.li.Init()
}

// Hash returns a hash of the given struct.
func Hash(v interface{}) string {
	q, err := hashstructure.Hash(v, nil)
	if err != nil {
		panic(fmt.Sprintf("Could not hash struct: ", err.Error()))
	}
	return strconv.FormatUint(q, 10)
}

type hash struct {
	name string
}

func (h *hash) Hash() string {
	return h.name
}

// String returns a Hashable that produces a hash equal to the given string.
func String(s string) Hashable {
	return &hash{s}
}
