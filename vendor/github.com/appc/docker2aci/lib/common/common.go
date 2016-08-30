// Copyright 2016 The appc Authors
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

// Package common provides misc types and variables.
package common

import (
	"github.com/appc/docker2aci/lib/internal/docker"
	"github.com/appc/docker2aci/lib/internal/types"
)

type Compression int

const (
	NoCompression = iota
	GzipCompression
)

type ParsedDockerURL types.ParsedDockerURL

const (
	AppcDockerRegistryURL   = "appc.io/docker/registryurl"
	AppcDockerRepository    = "appc.io/docker/repository"
	AppcDockerTag           = "appc.io/docker/tag"
	AppcDockerImageID       = "appc.io/docker/imageid"
	AppcDockerParentImageID = "appc.io/docker/parentimageid"
	AppcDockerEntrypoint    = "appc.io/docker/entrypoint"
	AppcDockerCmd           = "appc.io/docker/cmd"
)

type ErrSeveralImages struct {
	Msg    string
	Images []string
}

// InsecureConfig represents the different insecure options available
type InsecureConfig struct {
	SkipVerify bool
	AllowHTTP  bool
}

func (e *ErrSeveralImages) Error() string {
	return e.Msg
}

// ParseDockerURL takes a Docker URL and returns a ParsedDockerURL with its
// index URL, image name, and tag.
func ParseDockerURL(arg string) (*ParsedDockerURL, error) {
	p, err := docker.ParseDockerURL(arg)
	return (*ParsedDockerURL)(p), err
}
