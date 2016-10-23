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
	"net/url"
	"testing"
)

func TestDocker(t *testing.T) {
	tests := []struct {
		inDs        string
		inURIString string
		simpleStr   string
		err         error
	}{
		{
			"busybox",
			"cimd:docker:v=0:registry-1.docker.io/library/busybox:latest",
			"busybox",
			nil,
		},
		{
			"busybox:latest",
			"cimd:docker:v=0:registry-1.docker.io/library/busybox:latest",
			"busybox",
			nil,
		},
		{
			"registry-1.docker.io/library/busybox:latest",
			"cimd:docker:v=0:registry-1.docker.io/library/busybox:latest",
			"busybox",
			nil,
		},
		{
			"busybox:1.0",
			"cimd:docker:v=0:registry-1.docker.io/library/busybox:1.0",
			"busybox:1.0",
			nil,
		},
		{
			"repo/image",
			"cimd:docker:v=0:registry-1.docker.io/repo/image:latest",
			"repo/image",
			nil,
		},
		{
			"repo/image:latest",
			"cimd:docker:v=0:registry-1.docker.io/repo/image:latest",
			"repo/image",
			nil,
		},
		{
			"repo/image:1.0",
			"cimd:docker:v=0:registry-1.docker.io/repo/image:1.0",
			"repo/image:1.0",
			nil,
		},
		{
			"busybox@sha256:a59906e33509d14c036c8678d687bd4eec81ed7c4b8ce907b888c607f6a1e0e6",
			"cimd:docker:v=0:registry-1.docker.io/library/busybox@sha256:a59906e33509d14c036c8678d687bd4eec81ed7c4b8ce907b888c607f6a1e0e6",
			"busybox@sha256:a59906e33509d14c036c8678d687bd4eec81ed7c4b8ce907b888c607f6a1e0e6",
			nil,
		},
		{
			"myregistry.example.com:4000/busybox",
			"cimd:docker:v=0:myregistry.example.com:4000/busybox:latest",
			"myregistry.example.com:4000/busybox",
			nil,
		},
		{
			"myregistry.example.com:4000/busybox:latest",
			"cimd:docker:v=0:myregistry.example.com:4000/busybox:latest",
			"myregistry.example.com:4000/busybox",
			nil,
		},
		{
			"myregistry.example.com:4000/busybox:1.0",
			"cimd:docker:v=0:myregistry.example.com:4000/busybox:1.0",
			"myregistry.example.com:4000/busybox:1.0",
			nil,
		},
	}

	for _, tt := range tests {
		// Test NewDockerFromDockerString
		d, err := NewDockerFromDockerString(tt.inDs)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		u, err := url.Parse(tt.inURIString)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		td, err := NewDocker(u)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !d.Compare(td) {
			t.Fatalf("expected identical distribution but got %q != %q", td.URI().String(), d.URI().String())
		}
	}
}
