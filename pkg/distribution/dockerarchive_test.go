// Copyright 2018 The rkt Authors
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

func TestDockerArchive(t *testing.T) {
	tests := []struct {
		fileURL      string
		expectedCIMD string
	}{
		{
			"file:///full/path/to/busybox.tar",
			"cimd:docker-archive:v=0:file%3A%2F%2F%2Ffull%2Fpath%2Fto%2Fbusybox.tar",
		},
	}

	for _, tt := range tests {
		d, err := NewDockerArchiveFromString(tt.fileURL)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		u, err := url.Parse(tt.expectedCIMD)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		td, err := NewDockerArchive(u)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !d.Equals(td) {
			t.Fatalf("expected identical distribution but got %q != %q", td.CIMD().String(), d.CIMD().String())
		}
	}
}
