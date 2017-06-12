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
	"fmt"
	"testing"
)

var c *Cache

type cacheableT struct {
	Name string
}

func (ct *cacheableT) Hash() string {
	return Hash(ct)
}

var (
	key   = cacheableT{"foo"}
	value = "bar"
)

func TestNewCache(t *testing.T) {
	c = NewCache()
	if c == nil {
		t.Fatal("Expecting a new cache object.")
	}
}

func TestCacheReadNonExistentValue(t *testing.T) {
	if _, ok := c.Read(&key); ok {
		t.Fatal("Expecting false.")
	}
}

func TestCacheWritingValue(t *testing.T) {
	c.Write(&key, value)
	c.Write(&key, value)
}

func TestCacheReadExistentValue(t *testing.T) {
	s, ok := c.Read(&key)

	if !ok {
		t.Fatal("Expecting true.")
	}

	if s != value {
		t.Fatal("Expecting value.")
	}
}

func BenchmarkNewCache(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewCache()
	}
}

func BenchmarkReadNonExistentValue(b *testing.B) {
	z := NewCache()
	for i := 0; i < b.N; i++ {
		z.Read(&key)
	}
}

func BenchmarkWriteSameValue(b *testing.B) {
	z := NewCache()
	for i := 0; i < b.N; i++ {
		z.Write(&key, value)
	}
}

func BenchmarkWriteNewValue(b *testing.B) {
	z := NewCache()
	for i := 0; i < b.N; i++ {
		key := cacheableT{fmt.Sprintf("item-%d", i)}
		z.Write(&key, value)
	}
}

func BenchmarkReadExistentValue(b *testing.B) {
	z := NewCache()
	z.Write(&key, value)
	for i := 0; i < b.N; i++ {
		z.Read(&key)
	}
}
