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

// DistType represents a distribution type
type DistType string

const (
	DistScheme = "cimd"

	// Distribution types
	DistTypeAppc       DistType = "appc"
	DistTypeACIArchive DistType = "aci-archive"
	DistTypeDocker     DistType = "docker"
)

// A Distribution represnt the way to retrieve an image starting from an input
// string.
// It's identified by an URI with a specific schema
type Distribution interface {
	// Returns a copy of the distribution URI
	URI() *url.URL
	// Returns the distribution type
	Type() DistType
	// Returns a comparable URI string
	ComparableURIString() string
}

// NewDistribution creates a new distribution from the provided distribution uri
// string.
// It returns the right distribution based on the distribution type or an error
// if the uri string is wrong or referencing an unknown distribution type.
func NewDistribution(rawuri string) (Distribution, error) {
	u, err := url.Parse(rawuri)
	if err != nil {
		return nil, fmt.Errorf("cannot parse uri: %q: %v", rawuri, err)
	}
	distParts, err := ParseDist(u)
	if u.Scheme != DistScheme {
		return nil, fmt.Errorf("malformed distribution uri: %q", rawuri)
	}
	switch distParts.DistType {
	case "appc":
		return NewAppc(rawuri)
	case "aci-archive":
		return NewACIArchive(rawuri)
	case "docker":
		return NewDocker(rawuri)
	default:
		return nil, fmt.Errorf("unsupported distribution type: %q", distParts.DistType)
	}
}

// Equals returns true if the two distributions are identical
func Equals(d1 Distribution, d2 Distribution) bool {
	if d1.Type() != d2.Type() {
		return false
	}
	return d1.ComparableURIString() == d2.ComparableURIString()
}
