// Copyright 2016 The rkt Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package aci

import (
	"crypto/sha512"
	"fmt"
	"hash"
	"io"

	"github.com/appc/spec/discovery"
	"github.com/appc/spec/schema"
	"github.com/appc/spec/schema/types"
	"github.com/coreos/rkt/common/distribution"
	"github.com/coreos/rkt/store/casref/rwcasref"
	"github.com/coreos/rkt/store/manifestcache"
)

const (
	digestPrefix = "sha512-"
	lenHash      = sha512.Size // raw byte size
)

type ACIRegistry struct {
	s *rwcasref.Store
	c *manifestcache.ACIManifestCache
}

func NewACIRegistry(s *rwcasref.Store, c *manifestcache.ACIManifestCache) *ACIRegistry {
	return &ACIRegistry{s: s, c: c}
}

// GetACI retrieves the ACI digest that matches the provided app name and labels.
func (r *ACIRegistry) GetACI(name types.ACIdentifier, labels types.Labels) (string, error) {
	app, err := discovery.NewApp(name.String(), labels.ToMap())
	if err != nil {
		return "", err
	}
	refID := distribution.NewAppcFromApp(app).ComparableURIString()
	digest, err := r.s.GetRef(refID)
	if err != nil {
		return "", err
	}
	return digest, nil
}

func (r *ACIRegistry) GetImageManifest(key string) (*schema.ImageManifest, error) {
	return r.c.GetManifest(key)
}

func (r *ACIRegistry) ReadStream(key string) (io.ReadCloser, error) {
	return ReadACI(r.s, key)
}

func (r *ACIRegistry) ResolveKey(key string) (string, error) {
	return r.s.ResolveDigest(key)
}

// HashToKey takes a hash.Hash (which currently _MUST_ represent a full SHA512),
// calculates its sum, and returns a string which should be used as the key to
// store the data matching the hash.
func (r *ACIRegistry) HashToKey(h hash.Hash) string {
	s := h.Sum(nil)
	if len(s) != lenHash {
		panic(fmt.Sprintf("bad hash passed to hashToKey: %x", s))
	}
	return fmt.Sprintf("%s%x", digestPrefix, s)
}
