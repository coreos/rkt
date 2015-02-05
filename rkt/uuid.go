// Copyright 2015 CoreOS, Inc.
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

package main

import (
	"fmt"
	"strings"

	"github.com/coreos/rocket/Godeps/_workspace/src/github.com/appc/spec/schema/types"
)

// matchUUID attempts to match the uuid specified as uuid against all containers present.
// An array of matches is returned, which may be empty when nothing matches.
func matchUUID(uuid string) ([]string, error) {
	ls, err := listContainers(includeContainersDir | includeGarbageDir)
	if err != nil {
		return nil, err
	}

	var matches []string
	for _, c := range ls {
		if strings.HasPrefix(c, uuid) {
			matches = append(matches, c)
		}
	}

	return matches, nil
}

// resolveUUID attempts to resolve the uuid specified as uuid against all containers present.
// An unambiguously matched uuid or nil is returned.
func resolveUUID(uuid string) (*types.UUID, error) {
	uuid = strings.ToLower(uuid)
	m, err := matchUUID(uuid)
	if err != nil {
		return nil, err
	}

	if len(m) == 0 {
		return nil, fmt.Errorf("no matches found for %q", uuid)
	}

	if len(m) > 1 {
		return nil, fmt.Errorf("ambiguous uuid, %d matches", len(m))
	}

	u, err := types.NewUUID(m[0])
	if err != nil {
		return nil, fmt.Errorf("invalid UUID: %v", err)
	}

	return u, nil
}
