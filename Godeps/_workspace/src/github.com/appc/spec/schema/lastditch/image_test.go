// Copyright 2015 The appc Authors
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

package lastditch

import (
	"reflect"
	"testing"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/schema/types"
)

func TestImageManifestWithInvalidName(t *testing.T) {
	invalidName := "example.com/test!"
	imj := `
		{
		    "acKind": "ImageManifest",
		    "acVersion": "0.6.1",
		    "name": "` + invalidName + `"
		}
		`
	if types.ValidACIdentifier.MatchString(invalidName) {
		t.Fatalf("%q is an unexpectedly valid name", invalidName)
	}
	expected := ImageManifest{
		ACKind:    "ImageManifest",
		ACVersion: "0.6.1",
		Name:      invalidName,
	}
	im := ImageManifest{}
	if err := im.UnmarshalJSON([]byte(imj)); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(im, expected) {
		t.Errorf("did not get expected image manifest, got: %v, expected: %v", im, expected)
	}
}

func TestBogusImageManifest(t *testing.T) {
	bogus := []string{`
		{
		    "acKind": "Bogus",
		    "acVersion": "0.6.1",
		}
		`, `
		<html>
		    <head>
		        <title>Certainly not a JSON</title>
		    </head>
		</html>`,
	}

	for _, str := range bogus {
		im := ImageManifest{}
		if im.UnmarshalJSON([]byte(str)) == nil {
			t.Errorf("bogus image manifest unmarshalled successfully: %s", str)
		}
	}
}
