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

	d2acommon "github.com/appc/docker2aci/lib/common"
)

const (
	TypeDockerDaemon Type = "docker-daemon"
)

func init() {
	Register(TypeDockerDaemon, NewDockerDaemon)
}

type DockerDaemon struct {
	url       string // a valid docker reference URL
	parsedURL *d2acommon.ParsedDockerURL

	full   string // the full string representation for equals operations
	simple string // the user friendly (simple) string representation
}

func NewDockerDaemon(u *url.URL) (Distribution, error) {
	dp, err := parseCIMD(u)
	if err != nil {
		return nil, fmt.Errorf("cannot parse URI: %q: %v", u.String(), err)
	}
	if dp.Type != TypeDocker {
		return nil, fmt.Errorf("wrong distribution type: %q", dp.Type)
	}

	parsed, err := d2acommon.ParseDockerURL(dp.Data)
	if err != nil {
		return nil, fmt.Errorf("bad docker URL %q: %v", dp.Data, err)
	}

	return &DockerDaemon{
		url:       dp.Data,
		parsedURL: parsed,
		simple:    SimpleDockerRef(parsed),
		full:      FullDockerRef(parsed),
	}, nil
}

func NewDockerDaemonFromString(ds string) (Distribution, error) {
	urlStr := NewCIMDString(TypeDocker, distDockerVersion, ds)
	u, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}
	return NewDockerDaemon(u)
}

func (d *DockerDaemon) CIMD() *url.URL {
	uriStr := NewCIMDString(TypeDocker, distDockerVersion, d.url)
	// Create a copy of the URL
	u, err := url.Parse(uriStr)
	if err != nil {
		panic(err)
	}
	return u
}

func (d *DockerDaemon) String() string {
	return d.simple
}

func (d *DockerDaemon) Equals(dist Distribution) bool {
	d2, ok := dist.(*DockerDaemon)
	if !ok {
		return false
	}

	return d.full == d2.full
}

// ReferenceURL returns the docker reference URL.
func (d *DockerDaemon) ReferenceURL() string {
	return d.url
}
