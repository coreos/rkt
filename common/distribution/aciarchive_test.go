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

func TestACIArchive(t *testing.T) {
	tests := []struct {
		URL     string
		compStr string
		err     error
	}{
		{
			"file:///full/path/to/aci/file.aci",
			"cimd:aci-archive:v=0:file%3A%2F%2F%2Ffull%2Fpath%2Fto%2Faci%2Ffile.aci",
			nil,
		},
	}

	for _, tt := range tests {
		u, err := url.Parse(tt.URL)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		d, err := NewACIArchiveFromURL(u)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		outURL := d.ArchiveURL()
		if tt.URL != outURL.String() {
			t.Fatalf("expected url %q, but got %q", tt.URL, outURL.String())
		}
		compStr := d.ComparableURIString()
		if tt.compStr != compStr {
			t.Fatalf("expected comparable string %q, but got %q", tt.compStr, compStr)
		}
	}

}
