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
	"bytes"
	"fmt"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"

	d2acommon "github.com/appc/docker2aci/lib/common"
)

type distParts struct {
	DistType   DistType
	Version    uint32
	DistString string
}

// parseDist parses and returns the dist type, version and remaining part of a
// distribution URI
func parseDist(u *url.URL) (*distParts, error) {
	if u.Scheme != DistScheme {
		return nil, fmt.Errorf("unsupported scheme: %q", u.Scheme)
	}
	parts := strings.SplitN(u.Opaque, ":", 3)
	if len(parts) < 3 {
		return nil, fmt.Errorf("malformed distribution uri: %q", u.String())
	}
	version, err := strconv.ParseUint(strings.TrimPrefix(parts[1], "v="), 10, 32)
	if err != nil {
		return nil, fmt.Errorf("malformed distribution version: %s", parts[1])
	}
	return &distParts{
		DistType:   DistType(parts[0]),
		Version:    uint32(version),
		DistString: parts[2],
	}, nil
}

func DistBase(distType DistType, version uint32) string {
	return fmt.Sprintf("%s:%s:v=%d:", DistScheme, distType, version)
}

// from github.com/PuerkitoBio/purell
func sortQuery(u *url.URL) {
	q := u.Query()

	if len(q) > 0 {
		arKeys := make([]string, len(q))
		i := 0
		for k := range q {
			arKeys[i] = k
			i++
		}
		sort.Strings(arKeys)
		buf := new(bytes.Buffer)
		for _, k := range arKeys {
			sort.Strings(q[k])
			for _, v := range q[k] {
				if buf.Len() > 0 {
					buf.WriteRune('&')
				}
				buf.WriteString(fmt.Sprintf("%s=%s", k, url.QueryEscape(v)))
			}
		}

		// Rebuild the raw query string
		u.RawQuery = buf.String()
	}
}

// SimpleDockerString returns a simplyfied docker string. This means removing
// the index url if it's the default docker registry (registry-1.docker.io),
// removing the default repo (library) when using the default docker registry
// and removing the tag if it's "latest"
func SimpleDockerString(ds string) (string, error) {
	p, err := d2acommon.ParseDockerURL(ds)
	if err != nil {
		return "", fmt.Errorf("bad docker string %q: %v", ds, err)
	}

	var sds string
	if p.IndexURL != defaultIndexURL {
		sds += p.IndexURL
	}

	imageName := p.ImageName
	if p.IndexURL == defaultIndexURL && strings.HasPrefix(p.ImageName, defaultRepoPrefix) {
		imageName = strings.TrimPrefix(p.ImageName, defaultRepoPrefix)
	}

	if sds == "" {
		sds = imageName
	} else {
		sds = path.Join(sds, imageName)
	}

	digest := p.Digest
	tag := p.Tag
	if digest != "" {
		sds += "@" + digest
	} else {
		if tag != defaultTag {
			sds += ":" + tag
		}
	}
	return sds, nil
}

// FullDockerString return the docker uri populated with all the default values
// docker strings like "busybox" or
// "registry-1.docker.io/library/busybox:latest" will become the same docker
// string "registry-1.docker.io/library/busybox:latest"
func FullDockerString(ds string) (string, error) {
	p, err := d2acommon.ParseDockerURL(ds)
	if err != nil {
		return "", fmt.Errorf("bad docker string %q: %v", ds, err)
	}
	fds := path.Join(p.IndexURL, p.ImageName)
	digest := p.Digest
	tag := p.Tag
	if digest != "" {
		fds += "@" + digest
	} else {
		fds += ":" + tag
	}
	return fds, nil
}
