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
	defaultDirPerm  os.FileMode = 0660
	defaultFilePerm os.FileMode = 0660
	lockRetry                   = 3
)

// KeyLock is a lock for a specific key. The locks files are created inside a
// directory using the key name.
// The key name must be a valid file name.
type KeyLock struct {
	lockDir string
	key     string
	// The lock on the key
	keyLock *FileLock
}

// NewKeyLock returns a KeyLock for the specified key without acquisition.
// lockdir is the directory where the lock file will be created. If lockdir
// doesn't exists it will be created.
// The key name must be a valid file name.
func NewKeyLock(lockDir string, key string) (*KeyLock, error) {
	err := os.MkdirAll(lockDir, defaultDirPerm)
	if err != nil {
		return nil, err
	}
	keyLockFile := filepath.Join(lockDir, key)
	// create the file if it doesn't exists
	f, err := os.OpenFile(keyLockFile, os.O_RDONLY|os.O_CREATE, defaultFilePerm)
	if err != nil {
		return nil, fmt.Errorf("error creating key lock file: %v", err)
	}
	f.Close()
	keyLock, err := NewFileLock(keyLockFile)
	if err != nil {
		return nil, fmt.Errorf("error opening key lock file: %v", err)
	}
	return &KeyLock{lockDir: lockDir, key: key, keyLock: keyLock}, nil
}

// Close closes the key lock which implicitly unlocks it as well
func (l *KeyLock) Close() {
	l.keyLock.Close()
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
	var err error
	for retry := 0; retry < lockRetry; retry++ {
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

		if err == ErrFileChanged {
			l.keyLock.Close()
			nl, err := NewKeyLock(l.lockDir, l.key)
			if err != nil {
				return err
			}
			l.keyLock = nl.keyLock
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// Unlock unlocks the key lock and the lockDir lock.
func (l *KeyLock) Unlock() error {
	err := l.keyLock.Unlock()
	if err != nil {
		return err
	}
	return nil
}

// CleanKeyLocks remove lock files from the lockDir.
// For every key it tries to take an Exclusive lock on it and skip it if it
// fails with ErrLocked
func CleanKeyLocks(lockDir string) error {
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
		filename := filepath.Join(lockDir, f.Name())
		keyLock, err := TryExclusiveKeyLock(lockDir, f.Name())
		if err == ErrLocked {
			continue
		}
		if err != nil {
			return err
		}

		err = os.Remove(filename)
		if err != nil {
			keyLock.Close()
			return fmt.Errorf("error removing lock file: %v", err)
		}
		keyLock.Close()
	}
	return nil
}
