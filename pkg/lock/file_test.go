// Copyright 2014 The rkt Authors
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
	"runtime"
	"testing"
	"time"
)

func TestNewLock(t *testing.T) {
	f, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatalf("error creating tmpfile: %v", err)
	}
	defer os.Remove(f.Name())
	f.Close()

	l, err := NewLock(f.Name(), RegFile)
	if err != nil {
		t.Fatalf("error creating NewFileLock: %v", err)
	}
	l.Close()

	d, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("error creating tmpdir: %v", err)
	}
	defer os.Remove(d)

	l, err = NewLock(d, Dir)
	if err != nil {
		t.Fatalf("error creating NewLock: %v", err)
	}

	err = l.Close()
	if err != nil {
		t.Fatalf("error unlocking lock: %v", err)
	}

	if err = os.Remove(d); err != nil {
		t.Fatalf("error removing tmpdir: %v", err)
	}

	l, err = NewLock(d, Dir)
	if err == nil {
		t.Fatalf("expected error creating lock on nonexistent path")
	}
}

func TestExclusiveLock(t *testing.T) {
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("error creating tmpdir: %v", err)
	}
	defer os.Remove(dir)

	// Set up the initial exclusive lock
	l, err := ExclusiveLock(dir, Dir)
	if err != nil {
		t.Fatalf("error creating lock: %v", err)
	}

	// reacquire the exclusive lock using the receiver interface
	err = l.TryExclusiveLock()
	if err != nil {
		t.Fatalf("error reacquiring exclusive lock: %v", err)
	}

	// Now try another exclusive lock, should fail
	_, err = TryExclusiveLock(dir, Dir)
	if err == nil {
		t.Fatalf("expected err trying exclusive lock")
	}

	// Unlock the original lock
	err = l.Close()
	if err != nil {
		t.Fatalf("error closing lock: %v", err)
	}

	// Now another exclusive lock should succeed
	_, err = TryExclusiveLock(dir, Dir)
	if err != nil {
		t.Fatalf("error creating lock: %v", err)
	}
}

func TestSharedLock(t *testing.T) {
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("error creating tmpdir: %v", err)
	}
	defer os.Remove(dir)

	// Set up the initial shared lock
	l1, err := SharedLock(dir, Dir)
	if err != nil {
		t.Fatalf("error creating new shared lock: %v", err)
	}

	err = l1.TrySharedLock()
	if err != nil {
		t.Fatalf("error reacquiring shared lock: %v", err)
	}

	// Subsequent shared locks should succeed
	l2, err := TrySharedLock(dir, Dir)
	if err != nil {
		t.Fatalf("error creating shared lock: %v", err)
	}
	l3, err := TrySharedLock(dir, Dir)
	if err != nil {
		t.Fatalf("error creating shared lock: %v", err)
	}

	// But an exclusive lock should fail
	_, err = TryExclusiveLock(dir, Dir)
	if err == nil {
		t.Fatal("expected exclusive lock to fail")
	}

	// Close the locks
	err = l1.Close()
	if err != nil {
		t.Fatalf("error closing lock: %v", err)
	}
	err = l2.Close()
	if err != nil {
		t.Fatalf("error closing lock: %v", err)
	}

	// Only unlock one of them
	err = l3.Unlock()
	if err != nil {
		t.Fatalf("error unlocking lock: %v", err)
	}

	// Now try an exclusive lock, should succeed
	_, err = TryExclusiveLock(dir, Dir)
	if err != nil {
		t.Fatalf("error creating lock: %v", err)
	}
}

func TestVerifySameFile(t *testing.T) {
	testDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("error creating tmpdir: %v", err)
	}
	defer os.Remove(testDir)

	l, err := NewLock(testDir, Dir)
	if err != nil {
		t.Fatalf("error creating NewFileLock: %v", err)
	}
	defer l.Close()

	err = verifySameFile(l, testDir)
	if err != nil {
		t.Fatalf("error verifying that dir exists: %v", err)
	}

	err = os.Remove(testDir)
	if err != nil {
		t.Fatalf("error deleting dir: %v", err)
	}

	err = verifySameFile(l, testDir)
	if err != ErrNotExist {
		t.Fatalf("expected %v error got: %v", ErrNotExist, err)
	}
}

func TestFileDeletedBetweenLocks(t *testing.T) {
	testDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("error creating tmpdir: %v", err)
	}
	defer os.Remove(testDir)

	// Take exclusive lock on the file.
	excLock, err := ExclusiveLock(testDir, Dir)
	if err != nil {
		t.Fatalf("error creating exclusive lock: %v", err)
	}

	start := make(chan bool)
	finish := make(chan bool)
	go func() {
		close(start)
		defer close(finish)
		// This should block as exclusive lock is taken.
		// It should error because the file descriptor in lock
		// handle would be invalid once exclusive lock will be
		// released after deleting the testDir.
		_, err := SharedLock(testDir, Dir)
		if err == nil {
			t.Fatal("Should have received error in SharedLock")
		}
		if err != ErrNotExist {
			t.Fatalf("Expected %v error", ErrNotExist)
		}
	}()

	<-start
	// Let shared lock call be blocked.
	runtime.Gosched()
	time.Sleep(1 * time.Second)

	// Remove the file, SharedLock() inside above
	// goroutine should error.
	os.Remove(testDir)
	excLock.Close()

	<-finish
}

func TestFileRcreatedBetweenLocks(t *testing.T) {
	testDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("error creating tmpdir: %v", err)
	}
	defer os.Remove(testDir)

	excLock, err := TryExclusiveLock(testDir, Dir)
	if err != nil {
		t.Fatalf("error creating exclusive lock: %v", err)
	}

	start := make(chan bool)
	finish := make(chan bool)
	go func() {
		close(start)
		defer close(finish)

		// This should block as exclusive lock is taken.
		// It should error because the file descriptor in lock
		// handle would be invalid once exclusive lock will be
		// released after deleting and recreating the testDir.
		_, err := SharedLock(testDir, Dir)
		if err == nil {
			t.Fatal("should have received error in SharedLock")
		}
		if err != ErrNotExist {
			t.Fatalf("expected %v error", ErrNotExist)
		}
	}()

	<-start
	// Let shared lock call be blocked.
	runtime.Gosched()
	time.Sleep(1 * time.Second)

	// Delete and recreate the testDir to invalidate the FD
	// held by shared lock.
	os.Remove(testDir)
	err = os.Mkdir(testDir, os.FileMode(0755))
	if err != nil {
		t.Fatalf("error recreating the dir: %v", err)
	}
	excLock.Close()

	<-finish
}
