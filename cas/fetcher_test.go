// Copyright 2014 CoreOS, Inc.
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

package cas

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestNewFetcher(t *testing.T) {
	const (
		u1   = "https://example.com"
		u2   = "https://foo.com"
		data = "asdf"
	)
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	ds := NewStore(dir)

	// Create our first Fetcher, and simulate Store() to create index
	na := NewFetcher(u1, "")
	na.BlobKey = data
	ds.WriteIndex(na)

	// Create a new remote w the same parameters, reading from index should be fine
	nb := NewFetcher(u1, "")
	err = ds.ReadIndex(nb)
	if err != nil {
		t.Fatalf("unexpected error reading index: %v", err)
	}
	if nb.BlobKey != data {
		t.Fatalf("bad data returned from store: got %v, want %v", nb.BlobKey, data)
	}

	// Create a new remote with a different URI
	nc := NewFetcher(u2, "")
	err = ds.ReadIndex(nc)
	// Should get an error, since the URI shouldn't be indexed
	if err == nil {
		t.Errorf("unexpected nil error reading index")
	}
	// Fetcher shouldn't be populated
	if nc.BlobKey != "" {
		t.Errorf("unexpected blob: got %v", nc.BlobKey)
	}
}
