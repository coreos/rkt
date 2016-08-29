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
	"path"
	"strings"

	d2acommon "github.com/appc/docker2aci/lib/common"
)

const DistDockerVersion = 0

// Docker defines a distribution using a docker registry
// The format is the same as the docker image string format (man docker-pull)
// with the "docker" distribution type:
// cimd:docker:v=0:[REGISTRY_HOST[:REGISTRY_PORT]/]NAME[:TAG|@DIGEST]
// Examples:
// cimd:docker:v=0:busybox
// cimd:docker:v=0:busybox:latest
// cimd:docker:v=0:registry-1.docker.io/library/busybox@sha256:a59906e33509d14c036c8678d687bd4eec81ed7c4b8ce907b888c607f6a1e0e6
type Docker struct {
	ds string
}

// NewDocker creates a new docker distribution from the provided distribution uri string
func NewDocker(rawuri string) (*Docker, error) {
	u, err := url.Parse(rawuri)
	if err != nil {
		return nil, fmt.Errorf("cannot parse URI: %q: %v", rawuri, err)
	}
	dp, err := ParseDist(u)
	if err != nil {
		return nil, fmt.Errorf("cannot parse URI: %q: %v", rawuri, err)
	}
	if dp.DistType != DistTypeDocker {
		return nil, fmt.Errorf("wrong scheme: %q", dp.DistType)
	}

	if _, err = d2acommon.ParseDockerURL(dp.DistString); err != nil {
		return nil, fmt.Errorf("bad docker string %q: %v", dp.DistString, err)
	}
	return &Docker{ds: dp.DistString}, nil
}

// NewDocker creates a new docker distribution from the provided docker string
// (like "busybox", "busybox:1.0", "myregistry.example.com:4000/busybox"
// etc...)
func NewDockerFromDockerString(ds string) (*Docker, error) {
	return NewDocker(distBase(DistTypeDocker, DistDockerVersion) + ds)
}

func (d *Docker) Type() DistType {
	return DistTypeDocker
}

func (d *Docker) URI() *url.URL {
	uriStr := "docker:" + d.ds
	// Create a copy of the URL
	u, err := url.Parse(uriStr)
	if err != nil {
		panic(err)
	}
	return u
}

// ComparableURIString return the docker uri populated with all the default
// values to obtain a comparable string
// In this ways uri strings like docker:busybox or
// cimd:docker:v=0:registry-1.docker.io/library/busybox:latest
// will have the same comparable string:
// cimd:docker:v=0:registry-1.docker.io/library/busybox:latest
func (d *Docker) ComparableURIString() string {
	p, err := d2acommon.ParseDockerURL(d.ds)
	if err != nil {
		panic(fmt.Errorf("bad docker string %q: %v", d.ds, err))
	}

	uriStr := distBase(DistTypeDocker, DistDockerVersion) + path.Join(p.IndexURL, p.ImageName)

	digest := p.Digest
	tag := p.Tag
	if digest != "" {
		uriStr += "@" + digest
	} else {
		uriStr += ":" + tag
	}

	return uriStr
}

// DockerString returns the original Docker string
func (d *Docker) DockerString() string {
	return d.ds
}

// SimpleDockerString returns a simplyfied docker string. This means removing
// the index url if it's the default docker registry (registry-1.docker.io),
// removing the default repo (library) when using the default docker registry
// and removing the tag if it's "latest"
func (d *Docker) SimpleDockerString() string {
	p, err := d2acommon.ParseDockerURL(d.ds)
	if err != nil {
		panic(fmt.Errorf("bad docker string %q: %v", d.ds, err))
	}

	var ds string
	if p.IndexURL != defaultIndexURL {
		ds += p.IndexURL
	}

	imageName := p.ImageName
	if p.IndexURL == defaultIndexURL && strings.HasPrefix(p.ImageName, defaultRepoPrefix) {
		imageName = strings.TrimPrefix(p.ImageName, defaultRepoPrefix)
	}

	if ds == "" {
		ds = imageName
	} else {
		ds = path.Join(ds, imageName)
	}

	digest := p.Digest
	tag := p.Tag
	if digest != "" {
		ds += "@" + digest
	} else {
		if tag != defaultTag {
			ds += ":" + tag
		}
	}

	return ds
}

const (
	defaultIndexURL   = "registry-1.docker.io"
	defaultTag        = "latest"
	defaultRepoPrefix = "library/"
)
