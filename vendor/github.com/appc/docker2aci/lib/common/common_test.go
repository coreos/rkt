// Copyright 2017 The appc Authors
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

package common

import (
	"testing"
)

func TestMediaTypeSet(t *testing.T) {
	tests := []struct {
		ms                    MediaTypeSet
		expectedManifestTypes []string
		expectedConfigTypes   []string
		expectedLayerTypes    []string
	}{
		{
			MediaTypeSet{MediaTypeOptionDockerV21},
			[]string{MediaTypeDockerV21Manifest},
			[]string{},
			[]string{},
		},
		{
			MediaTypeSet{MediaTypeOptionDockerV22},
			[]string{MediaTypeDockerV22Manifest},
			[]string{MediaTypeDockerV22Config},
			[]string{MediaTypeDockerV22RootFS},
		},
		{
			MediaTypeSet{MediaTypeOptionOCIV1Pre},
			[]string{MediaTypeOCIV1Manifest},
			[]string{MediaTypeOCIV1Config},
			[]string{MediaTypeOCIV1Layer},
		},
		{
			MediaTypeSet{},
			[]string{MediaTypeDockerV21Manifest, MediaTypeDockerV22Manifest, MediaTypeOCIV1Manifest},
			[]string{MediaTypeDockerV22Config, MediaTypeOCIV1Config},
			[]string{MediaTypeDockerV22RootFS, MediaTypeOCIV1Layer},
		},
		{
			MediaTypeSet{MediaTypeOptionDockerV21, MediaTypeOptionDockerV22, MediaTypeOptionOCIV1Pre},
			[]string{MediaTypeDockerV21Manifest, MediaTypeDockerV22Manifest, MediaTypeOCIV1Manifest},
			[]string{MediaTypeDockerV22Config, MediaTypeOCIV1Config},
			[]string{MediaTypeDockerV22RootFS, MediaTypeOCIV1Layer},
		},
		{
			MediaTypeSet{MediaTypeOptionDockerV21, MediaTypeOptionOCIV1Pre},
			[]string{MediaTypeDockerV21Manifest, MediaTypeOCIV1Manifest},
			[]string{MediaTypeOCIV1Config},
			[]string{MediaTypeOCIV1Layer},
		},
	}

	for _, test := range tests {
		if !isEqual(test.expectedManifestTypes, test.ms.ManifestMediaTypes()) {
			t.Errorf("expected manifest media types didn't match what was returned:\n%v\n%v", test.expectedManifestTypes, test.ms.ManifestMediaTypes())
		}
		if !isEqual(test.expectedConfigTypes, test.ms.ConfigMediaTypes()) {
			t.Errorf("expected config media types didn't match what was returned:\n%v\n%v", test.expectedConfigTypes, test.ms.ConfigMediaTypes())
		}
		if !isEqual(test.expectedLayerTypes, test.ms.LayerMediaTypes()) {
			t.Errorf("expected layer media types didn't match what was returned:\n%v\n%v", test.expectedLayerTypes, test.ms.LayerMediaTypes())
		}
	}
}

func TestRegistryOptionSet(t *testing.T) {
	tests := []struct {
		rs       RegistryOptionSet
		allowsV1 bool
		allowsV2 bool
	}{
		{
			RegistryOptionSet{RegistryOptionV1}, true, false,
		},
		{
			RegistryOptionSet{RegistryOptionV2}, false, true,
		},
		{
			RegistryOptionSet{RegistryOptionV1, RegistryOptionV2}, true, true,
		},
		{
			RegistryOptionSet{}, true, true,
		},
	}
	for _, test := range tests {
		if test.allowsV1 != test.rs.AllowsV1() {
			t.Errorf("doesn't allow V1 when it should")
		}
		if test.allowsV2 != test.rs.AllowsV2() {
			t.Errorf("doesn't allow V1 when it should")
		}
	}
}

func isEqual(val1, val2 []string) bool {
	if len(val1) != len(val2) {
		return false
	}
loop1:
	for _, thing1 := range val1 {
		for _, thing2 := range val2 {
			if thing1 == thing2 {
				continue loop1
			}
		}
		return false
	}
	return true
}
