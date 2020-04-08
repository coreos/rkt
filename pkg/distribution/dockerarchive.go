// Copyright 2018 The rkt Authors
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

	"github.com/PuerkitoBio/purell"
)

const (
	distDockerArchiveVersion = 0

	// TypeDocker represents the DockerArchive distribution type
	TypeDockerArchive Type = "docker-archive"
)

func init() {
	Register(TypeDockerArchive, NewDockerArchive)
}

// DockerArchive defines a distribution using local docker tarball
// The format is:
// cimd:docker-archive:v=0:ArchivePath...
// ArchivePath must be query escaped
// Examples:
// cimd:docker-archive:v=0:file%3A%2F%2Fabsolute%2Fpath%2Fto%2Ffile
type DockerArchive struct {
	cimdURL *url.URL // the cimd URL as explained in the examples
	fileURL string   // the local path of the target docker archive, i.e. file://path/to/file.tar
}

// NewDockerArchive creates a new docker-archive distribution from the provided uri string
func NewDockerArchive(u *url.URL) (Distribution, error) {
	dp, err := parseCIMD(u)
	if err != nil {
		return nil, fmt.Errorf("cannot parse URI: %q: %v", u.String(), err)
	}
	if dp.Type != TypeDockerArchive {
		return nil, fmt.Errorf("wrong distribution type: %q", dp.Type)
	}

	path, err := url.QueryUnescape(dp.Data)

	// save the URI as sorted to make it ready for comparison
	purell.NormalizeURL(u, purell.FlagSortQuery)

	return &DockerArchive{
		cimdURL: u,
		fileURL: path,
	}, nil
}

// NewDockerArchiveFromString creates a new docker-archive distribution from the provided
// tarball path (like "some/path/to/busybox.tar" etc...)
func NewDockerArchiveFromString(ds string) (Distribution, error) {
	urlStr := NewCIMDString(TypeDockerArchive, distDockerArchiveVersion, url.QueryEscape(ds))
	u, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}
	return NewDockerArchive(u)
}

func (d *DockerArchive) CIMD() *url.URL {
	// Create a copy of the URL.
	u, err := url.Parse(d.cimdURL.String())
	if err != nil {
		panic(err)
	}
	return u
}

func (d *DockerArchive) Equals(dist Distribution) bool {
	a2, ok := dist.(*DockerArchive)
	if !ok {
		return false
	}

	return d.CIMD().String() == a2.CIMD().String()
}

func (d *DockerArchive) String() string {
	return d.fileURL
}
