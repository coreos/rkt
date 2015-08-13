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
)

func TestInvalidPodManifest(t *testing.T) {
	// empty image JSON
	eImgJ := "{}"
	// empty image instance
	eImgI := imgI("", "")
	tests := []struct {
		desc     string
		json     string
		expected PodManifest
	}{
		{
			desc:     "Check an empty pod manifest",
			json:     podJ("", ""),
			expected: podI(AppList{}),
		},
		{
			desc:     "Check a pod manifest with an invalid app name",
			json:     podJ(appJ("!", eImgJ, ""), ""),
			expected: podI(AppList{appI("!", eImgI)}),
		},
		{
			desc:     "Check a pod manifest with duplicated app names",
			json:     podJ(appJ("a", eImgJ, "")+","+appJ("a", eImgJ, ""), ""),
			expected: podI(AppList{appI("a", eImgI), appI("a", eImgI)}),
		},
		{
			desc:     "Check a pod manifest with an invalid image name and ID",
			json:     podJ(appJ("?", imgJ("!!!", "&&&", ""), ""), ""),
			expected: podI(AppList{appI("?", imgI("!!!", "&&&"))}),
		},
		{
			desc:     "Check if we ignore extra fields in a pod",
			json:     podJ("", `"ports": [],`),
			expected: podI(AppList{}),
		},
		{
			desc:     "Check if we ignore extra fields in an app",
			json:     podJ(appJ("a", eImgJ, `"mounts": [],`), `"ports": [],`),
			expected: podI(AppList{appI("a", eImgI)}),
		},
		{
			desc:     "Check if we ignore extra fields in an image",
			json:     podJ(appJ("a", imgJ("i", "id", `"labels": [],`), `"mounts": [],`), `"ports": [],`),
			expected: podI(AppList{appI("a", imgI("i", "id"))}),
		},
	}
	for _, tt := range tests {
		got := PodManifest{}
		if err := got.UnmarshalJSON([]byte(tt.json)); err != nil {
			t.Errorf("%s: unexpected error during unmarshalling pod manifest: %v", tt.desc, err)
		}
		if !reflect.DeepEqual(tt.expected, got) {
			t.Errorf("%s: did not get expected pod manifest, got: %v, expected: %v", tt.desc, got, tt.expected)
		}
	}
}

func TestBogusPodManifest(t *testing.T) {
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
		pm := PodManifest{}
		if pm.UnmarshalJSON([]byte(str)) == nil {
			t.Errorf("bogus pod manifest unmarshalled successfully: %s", str)
		}
	}
}

// podJ returns a pod manifest JSON with given apps
func podJ(apps, extra string) string {
	return `
		{
		    ` + extra + `
		    "acKind": "PodManifest",
		    "acVersion": "0.6.1",
		    "apps": [` + apps + `]
		}`
}

// podI returns a pod manifest instance with given apps
func podI(apps AppList) PodManifest {
	return PodManifest{
		ACVersion: "0.6.1",
		ACKind:    "PodManifest",
		Apps:      apps,
	}
}

// appJ returns an app JSON snippet with given name and image
func appJ(name, image, extra string) string {
	return `
		{
		    ` + extra + `
		    "name": "` + name + `",
		    "image": ` + image + `
		}`
}

// appI returns an app instance with given name and image
func appI(name string, image Image) RuntimeApp {
	return RuntimeApp{
		Name:  name,
		Image: image,
	}
}

// imgJ returns an image JSON snippet with given name and id
func imgJ(name, id, extra string) string {
	return `
		{
		    ` + extra + `
		    "name": "` + name + `",
		    "id": "` + id + `"
		}`
}

// imgI returns an image instance with given name and id
func imgI(name, id string) Image {
	return Image{
		Name: name,
		ID:   id,
	}
}
