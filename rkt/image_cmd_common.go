// Copyright 2015 The rkt Authors
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
	"errors"
	"fmt"

	pkgdigest "github.com/coreos/rkt/pkg/digest"
	"github.com/coreos/rkt/rkt/image"
	"github.com/coreos/rkt/store/casref/rwcasref"

	"github.com/hashicorp/errwrap"
)

func getDigestFromRef(s *rwcasref.Store, refstring string) (string, error) {
	d, err := image.DistFromImageString(refstring)
	if err != nil {
		return "", errwrap.Wrap(fmt.Errorf("cannot find image with reference %q", refstring), err)
	}
	digest, err := s.GetRef(d.ComparableURIString())
	if err != nil {
		return "", errwrap.Wrap(fmt.Errorf("cannot find image with reference %q", refstring), err)
	}
	return digest, nil
}

func getDigestFromRefOrDigest(s *rwcasref.Store, input string) (string, error) {
	var d string
	if _, _, err := pkgdigest.ParseDigest(input); err == nil {
		d, err = s.ResolveDigest(input)
		if err != nil {
			return "", errwrap.Wrap(errors.New("cannot resolve image digest"), err)
		}
	} else {
		d, err = getDigestFromRef(s, input)
		if err != nil {
			return "", errwrap.Wrap(errors.New("cannot find image"), err)
		}
	}
	return d, nil
}
