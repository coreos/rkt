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

package distribution

import (
	"fmt"
	"net/url"
)

// DistType represents the Distribution type
type DistType string

const (
	// DistScheme represents the Distribution URI scheme
	DistScheme = "cimd"
)

// A Distribution represent the way to retrieve an image starting from an input
// string.
// It's identified by an URI with a specific schema
type Distribution interface {
	// URI returns a copy of the Distribution URI
	URI() *url.URL
	// Type returns the distribution type
	Type() DistType
	// Compare compares with another Distribution
	Compare(Distribution) bool
}

type newDistribution func(*url.URL) (Distribution, error)

var distributions = make(map[DistType]newDistribution)

// Register registers a function that returns a new instance of the given
// distribution. This is intended to be called from the init function in
// packages that implement distribution functions.
func Register(distType DistType, f newDistribution) {
	if _, ok := distributions[distType]; ok {
		panic(fmt.Errorf("distribution %q already registered", distType))
	}
	distributions[distType] = f
}

// New returns a Distribution from the input URI.
// It returns an error if the uri string is wrong or referencing an unknown
// distribution type.
func New(u *url.URL) (Distribution, error) {
	dp, err := parseDist(u)
	if err != nil {
		return nil, fmt.Errorf("malformed distribution uri %q: %v", u.String(), err)
	}
	if u.Scheme != DistScheme {
		return nil, fmt.Errorf("malformed distribution uri %q", u.String())
	}
	if _, ok := distributions[dp.DistType]; !ok {
		return nil, fmt.Errorf("unknown distribution type: %q", dp.DistType)
	}
	return distributions[dp.DistType](u)
}

// Parse parses the provided distribution URI string and returns a
// Distribution.
func Parse(rawuri string) (Distribution, error) {
	u, err := url.Parse(rawuri)
	if err != nil {
		return nil, fmt.Errorf("cannot parse uri: %q: %v", rawuri, err)
	}
	return New(u)
}

// Type returns the distribution type
func Type(d Distribution) (DistType, error) {
	dt := d.Type()
	if _, ok := distributions[dt]; !ok {
		return "", fmt.Errorf("unknown distribution type: %q", dt)
	}
	return dt, nil
}

// Compare returns true if the two provided distributions are identical
func Compare(d1 Distribution, d2 Distribution) bool {
	return d1.Compare(d2)
}
