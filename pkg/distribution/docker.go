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

	d2acommon "github.com/appc/docker2aci/lib/common"
)

const (
	distDockerVersion = 0

	// DistTypeDocker represents the Docker distribution type
	DistTypeDocker DistType = "docker"

	defaultIndexURL   = "registry-1.docker.io"
	defaultTag        = "latest"
	defaultRepoPrefix = "library/"
)

func init() {
	Register(DistTypeDocker, NewDocker)
}

// Docker defines a distribution using a docker registry.
// The format after the docker distribution type prefix (cimd:docker:v=0:) is the same
// as the docker image string format (man docker-pull):
// cimd:docker:v=0:[REGISTRY_HOST[:REGISTRY_PORT]/]NAME[:TAG|@DIGEST]
// Examples:
// cimd:docker:v=0:busybox
// cimd:docker:v=0:busybox:latest
// cimd:docker:v=0:registry-1.docker.io/library/busybox@sha256:a59906e33509d14c036c8678d687bd4eec81ed7c4b8ce907b888c607f6a1e0e6
type Docker struct {
	ds string
}

// NewDocker creates a new docker distribution from the provided distribution uri
// string
func NewDocker(u *url.URL) (Distribution, error) {
	dp, err := parseDist(u)
	if err != nil {
		return nil, fmt.Errorf("cannot parse URI: %q: %v", u.String(), err)
	}
	if dp.DistType != DistTypeDocker {
		return nil, fmt.Errorf("wrong distribution type: %q", dp.DistType)
	}

	if _, err = d2acommon.ParseDockerURL(dp.DistString); err != nil {
		return nil, fmt.Errorf("bad docker string %q: %v", dp.DistString, err)
	}
	return &Docker{ds: dp.DistString}, nil
}

// NewDockerFromDockerString creates a new docker distribution from the provided
// docker string (like "busybox", "busybox:1.0", "myregistry.example.com:4000/busybox"
// etc...)
func NewDockerFromDockerString(ds string) (Distribution, error) {
	urlStr := DistBase(DistTypeDocker, distDockerVersion) + ds
	u, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}
	return NewDocker(u)
}

// URI returns a copy of the Distribution URI
func (d *Docker) URI() *url.URL {
	uriStr := DistBase(DistTypeDocker, distDockerVersion) + d.ds
	// Create a copy of the URL
	u, err := url.Parse(uriStr)
	if err != nil {
		panic(err)
	}
	return u
}

// Type returns the Distribution type
func (d *Docker) Type() DistType {
	return DistTypeDocker
}

// Compare compares with another Distribution
func (d *Docker) Compare(dist Distribution) bool {
	d2, ok := dist.(*Docker)
	if !ok {
		return false
	}
	fds1, err := FullDockerString(d.ds)
	if err != nil {
		panic(err)
	}
	fds2, err := FullDockerString(d2.ds)
	if err != nil {
		panic(err)
	}
	return fds1 == fds2
}

// DockerString returns the docker string.
func (d *Docker) DockerString() string {
	return d.ds
}
