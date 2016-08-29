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

const DistACIArchiveVersion = 0

// ACIArchive defines a distribution using an ACI file
// The format is:
// cmd:aci-archive:v=0:ArchiveURL?query...
// The distribution type is "archive"
// ArchiveURL must be query escaped
// Examples:
// cimd:aci-archive:v=0:file%3A%2F%2Fabsolute%2Fpath%2Fto%2Ffile
// cimd:aci-archive:v=0:https%3A%2F%2Fexample.com%2Fapp.aci
type ACIArchive struct {
	u *url.URL
	// The transport url
	tu *url.URL
}

// NewACIArchive creates a new aci-archive distribution from the provided distribution uri
// string
func NewACIArchive(rawuri string) (*ACIArchive, error) {
	u, err := url.Parse(rawuri)
	if err != nil {
		return nil, fmt.Errorf("cannot parse URI: %q: %v", rawuri, err)
	}
	dp, err := ParseDist(u)
	if err != nil {
		return nil, fmt.Errorf("cannot parse URI: %q: %v", rawuri, err)
	}
	if dp.DistType != DistTypeACIArchive {
		return nil, fmt.Errorf("wrong scheme: %q", u.Scheme)
	}
	// This should be a valid URL
	tus, err := url.QueryUnescape(dp.DistString)
	if err != nil {
		return nil, fmt.Errorf("wrong archive transport url %q: %v", dp.DistString, err)
	}
	tu, err := url.Parse(tus)
	if err != nil {
		return nil, fmt.Errorf("wrong archive transport url %q: %v", dp.DistString, err)
	}

	// sort the query for reproducible strings
	sortQuery(u)

	return &ACIArchive{u: u, tu: tu}, nil
}

// NewACIArchive creates a new aci-archive distribution from the provided transport URL
// Example: file:///full/path/to/aci/file.aci -> archive:aci:file%3A%2F%2F%2Ffull%2Fpath%2Fto%2Faci%2Ffile.aci
func NewACIArchiveFromURL(u *url.URL) (*ACIArchive, error) {
	return NewACIArchive(distBase(DistTypeACIArchive, DistACIArchiveVersion) + url.QueryEscape(u.String()))
}

func (a *ACIArchive) Type() DistType {
	return DistTypeACIArchive
}

func (a *ACIArchive) ComparableURIString() string {
	return a.u.String()
}

func (a *ACIArchive) URI() *url.URL {
	// Create a copy of the URL
	u, err := url.Parse(a.u.String())
	if err != nil {
		panic(err)
	}
	return u
}

// ArchiveURL returns a copy of the transport URL
func (a *ACIArchive) ArchiveURL() *url.URL {
	// Create a copy of the transport URL
	tu, err := url.Parse(a.tu.String())
	if err != nil {
		panic(err)
	}
	return tu
}
