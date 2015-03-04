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
	"fmt"
	"os"
	"path/filepath"
)

const (
	defaultPathPerm os.FileMode = 0777
)

// KeyLock is a lock for a specific key. The locks files (actually they are
// directories as DirLock is used) are created inside a directory.
// As a lock file is created for every key, there's the need to remove old lock
// files (for example by a garbage collection function).
// It's not possible to just remove a lock file taking an exclusive lock
// on it as there can be other users waiting for a lock on it and if the file
// is removed from the filesystem these users will have its fd open and will
// continue to take/wait for a lock while new users will create a new lock file
// creating a race condition.
// For this reason the image lock file removal needs to be done taking an
// exclusive lock on the directory containing the lock
type KeyLock struct {
	// The lock on the lock dir
	lockDirLock *DirLock
	// The lock on the key
	keyLock *DirLock
}

// NewKeyLock returns a KeyLock for the specified key without acquisition.
// lockdir is the directory where the lock file will be created.
func NewKeyLock(lockDir string, key string) (*KeyLock, error) {
	err := os.MkdirAll(lockDir, defaultPathPerm)
	if err != nil {
		return nil, err
	}
	lockDirLock, err := NewLock(lockDir)
	if err != nil {
		return nil, fmt.Errorf("error opening lockDir: %v", err)
	}
	keyLockFile := filepath.Join(lockDir, key)
	err = os.MkdirAll(keyLockFile, defaultPathPerm)
	if err != nil {
		return nil, fmt.Errorf("error creating key lock file: %v", err)
	}
	keyLock, err := NewLock(keyLockFile)
	if err != nil {
		return nil, fmt.Errorf("error opening key lock file: %v", err)
	}
	return &KeyLock{lockDirLock: lockDirLock, keyLock: keyLock}, nil
}

// Close closes the key lock which implicitly unlocks it as well
func (l *KeyLock) Close() {
	l.keyLock.Close()
	l.lockDirLock.Close()
}

// TryExclusiveLock takes an exclusive lock on a key without blocking.
// This is idempotent when the KeyLock already represents an exclusive lock,
// and tries promote a shared lock to exclusive atomically.
// It will return ErrLocked if any lock is already held on the key.
func (l *KeyLock) TryExclusiveKeyLock() error {
	return l.lock(true, true)
}

// TryExclusiveLock takes an exclusive lock on the key without blocking.
// lockDir is the directory where the lock file will be created.
// It will return ErrLocked if any lock is already held.
func TryExclusiveKeyLock(lockDir string, key string) (*KeyLock, error) {
	return createAndLock(lockDir, key, true, true)
}

// ExclusiveLock takes an exclusive lock on a key.
// This is idempotent when the KeyLock already represents an exclusive lock,
// and promotes a shared lock to exclusive atomically.
// It will block if an exclusive lock is already held on the key.
func (l *KeyLock) ExclusiveKeyLock() error {
	return l.lock(false, true)
}

// ExclusiveLock takes an exclusive lock on a key.
// lockDir is the directory where the lock file will be created.
// It will block if an exclusive lock is already held on the key.
func ExclusiveKeyLock(lockDir string, key string) (*KeyLock, error) {
	return createAndLock(lockDir, key, false, true)
}

// TrySharedLock takes a co-operative (shared) lock on the key without blocking.
// This is idempotent when the KeyLock already represents a shared lock,
// and tries demote an exclusive lock to shared atomically.
// It will return ErrLocked if an exclusive lock already exists on the key.
func (l *KeyLock) TrySharedKeyLock() error {
	return l.lock(true, false)
}

// TrySharedLock takes a co-operative (shared) lock on a key without blocking.
// lockDir is the directory where the lock file will be created.
// It will return ErrLocked if an exclusive lock already exists on the key.
func TrySharedKeyLock(lockDir string, key string) (*KeyLock, error) {
	return createAndLock(lockDir, key, true, false)
}

// SharedLock takes a co-operative (shared) lock on a key.
// This is idempotent when the KeyLock already represents a shared lock,
// and demotes an exclusive lock to shared atomically.
// It will block if an exclusive lock is already held on the key.
func (l *KeyLock) SharedKeyLock() error {
	return l.lock(false, false)
}

// SharedLock takes a co-operative (shared) lock on a key.
// lockDir is the directory where the lock file will be created.
// It will block if an exclusive lock is already held on the key.
func SharedKeyLock(lockDir string, key string) (*KeyLock, error) {
	return createAndLock(lockDir, key, false, false)
}

func createAndLock(lockDir string, key string, try bool, exclusive bool) (*KeyLock, error) {
	keyLock, err := NewKeyLock(lockDir, key)
	if err != nil {
		return nil, err
	}
	err = keyLock.lock(try, exclusive)
	if err != nil {
		return nil, err
	}
	return keyLock, nil
}

func (l *KeyLock) lock(try bool, exclusive bool) error {
	// First take a shared lock on LockDir
	err := l.lockDirLock.SharedLock()
	if err != nil {
		return fmt.Errorf("error locking lockDir: %v", err)
	}

	switch {
	case exclusive && !try:
		err = l.keyLock.ExclusiveLock()
	case exclusive && try:
		err = l.keyLock.TryExclusiveLock()
	case !exclusive && !try:
		err = l.keyLock.SharedLock()
	case !exclusive && try:
		err = l.keyLock.TrySharedLock()
	}
	if err != nil {
		l.lockDirLock.Unlock()
		return err
	}

	return nil
}

// Unlock unlocks the key lock and the lockDir lock.
func (l *KeyLock) Unlock() error {
	err := l.keyLock.Unlock()
	if err != nil {
		return err
	}
	err = l.lockDirLock.Unlock()
	if err != nil {
		return err
	}
	return nil
}

// CleanKeyLocks remove lock files from the lockDir. If try is true then it
// tries to take an Exclusive lock on lockDir and if a lock lockDir is already
// held it will exit with ErrLocked. If try is false than it'll wait
// undefinitely for an exclusive lock on lockDir.
func CleanKeyLocks(lockDir string, try bool) error {
	// First take an exclusive lock on the lockDir
	var lockDirLock *DirLock
	var err error
	if try {
		lockDirLock, err = TryExclusiveLock(lockDir)
	} else {
		lockDirLock, err = ExclusiveLock(lockDir)
	}
	if err != nil {
		return err
	}
	defer lockDirLock.Close()

	f, err := os.Open(lockDir)
	if err != nil {
		return fmt.Errorf("error opening lockDir: %v", err)
	}
	defer f.Close()
	files, err := f.Readdir(0)
	if err != nil {
		return fmt.Errorf("error getting lock files list: %v", err)
	}
	for _, f := range files {
		err := os.Remove(filepath.Join(lockDir, f.Name()))
		if err != nil {
			return fmt.Errorf("error removing lock file: %v", err)
		}

	}
	return nil
}
