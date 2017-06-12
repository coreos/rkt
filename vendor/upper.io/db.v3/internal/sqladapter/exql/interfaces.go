package exql

import (
	"upper.io/db.v3/internal/cache"
)

// Fragment is any interface that can be both cached and compiled.
type Fragment interface {
	cache.Hashable
	compilable
}

type compilable interface {
	Compile(*Template) (string, error)
}
