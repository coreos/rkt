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

package lock

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestExclusiveKeyLock(t *testing.T) {
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("error creating tmpdir: %v", err)
	}
	defer os.Remove(dir)

	l1, err := ExclusiveKeyLock(dir, "key01")
	if err != nil {
		t.Fatalf("error creating key lock: %v", err)
	}

	_, err = TryExclusiveKeyLock(dir, "key01")
	if err == nil {
		t.Fatalf("expected err trying exclusive key lock")
	}

	l1.Close()
}

func TestCleanKeyLocks(t *testing.T) {
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("error creating tmpdir: %v", err)
	}
	defer os.Remove(dir)

	l1, err := ExclusiveKeyLock(dir, "key01")
	if err != nil {
		t.Fatalf("error creating keyLock: %v", err)
	}

	err = CleanKeyLocks(dir, true)
	if err == nil {
		t.Fatalf("expected err trying CleanKeyLocks")
	}

	l2, err := SharedKeyLock(dir, "key02")
	if err != nil {
		t.Fatalf("error creating keyLock: %v", err)
	}

	l1.Close()
	l2.Close()

	err = CleanKeyLocks(dir, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	f, err := os.Open(dir)
	if err != nil {
		t.Fatalf("error opening lockDir: %v", err)
	}
	defer f.Close()
	files, err := f.Readdir(0)
	if len(files) != 0 {
		t.Fatalf("exptected empty lock dir. found %d files: %s", len(files), files)
	}
}
