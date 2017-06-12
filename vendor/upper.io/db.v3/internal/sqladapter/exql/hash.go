package exql

import (
	"reflect"
	"sync/atomic"

	"upper.io/db.v3/internal/cache"
)

type hash struct {
	v atomic.Value
}

func (h *hash) Hash(i interface{}) string {
	v := h.v.Load()
	if r, ok := v.(string); ok && r != "" {
		return r
	}
	s := reflect.TypeOf(i).String() + ":" + cache.Hash(i)
	h.v.Store(s)
	return s
}

func (h *hash) Reset() {
	h.v.Store("")
}
