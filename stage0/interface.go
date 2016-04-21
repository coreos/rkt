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

//+build linux

package stage0

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"strconv"

	"github.com/appc/spec/schema"
	"github.com/coreos/rkt/common"
	"github.com/hashicorp/errwrap"
)

const (
	interfaceVersion = "coreos.com/rkt/stage1/interface-version"
)

type ifaceVersion int

// getStage1InterfaceVersion retrieves the interface version from the stage1
// manifest for a given pod
func getStage1InterfaceVersion(cdir string) (ifaceVersion, error) {
	var s1v ifaceVersion
	s1v.Set(-1)
	b, err := ioutil.ReadFile(common.Stage1ManifestPath(cdir))
	if err != nil {
		return s1v, errwrap.Wrap(errors.New("error reading pod manifest"), err)
	}

	s1m := schema.ImageManifest{}
	if err := json.Unmarshal(b, &s1m); err != nil {
		return s1v, errwrap.Wrap(errors.New("error unmarshaling stage1 manifest"), err)
	}

	if iv, ok := s1m.Annotations.Get(interfaceVersion); ok {
		v, err := strconv.Atoi(iv)
		if err != nil {
			return s1v, errwrap.Wrap(errors.New("error parsing interface version"), err)
		}
		s1v.Set(v)
		return s1v, nil
	}

	// "interface-version" annotation not found, assume version 1
	s1v.Set(1)
	return s1v, nil
}

func (s1v *ifaceVersion) Set(v int) {
	*(*int)(s1v) = v
}

func (s1v *ifaceVersion) Get() int {
	return *(*int)(s1v)
}

func (s1v *ifaceVersion) SupportsHostname() bool {
	return s1v.Get() > 1
}

func (s1v *ifaceVersion) SupportsMoreConfigDirectories() bool {
	return s1v.Get() > 2
}
