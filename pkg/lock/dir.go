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

// Package lock implements simple locking primitives on a
// regular file or directory using flock
package lock

import (
	"errors"
	"syscall"
)

var (
	ErrLocked      = errors.New("file already locked")
	ErrNotExist    = errors.New("file does not exist")
	ErrPermission  = errors.New("permission denied")
	ErrNotRegular  = errors.New("not a regular file")
	ErrFileChanged = errors.New("lock file changed")
)

// FileLock represents a lock on a regular file or a directory
type FileLock struct {
	path string
	fd   int
	stat syscall.Stat_t
}

// TryExclusiveLock takes an exclusive lock without blocking.
// This is idempotent when the Lock already represents an exclusive lock,
// and tries promote a shared lock to exclusive atomically.
// It will return ErrLocked if any lock is already held.
func (l *FileLock) TryExclusiveLock() error {
	return l.Lock(true, true)
}

// TryExclusiveRegFileLock takes an exclusive lock on a file without blocking.
// It will return ErrLocked if any lock is already held on the file.
func TryExclusiveRegFileLock(path string) (*FileLock, error) {
	return TryExclusiveLock(path, false)
}

// TryExclusiveDirLock takes an exclusive lock on a directory without blocking.
// It will return ErrLocked if any lock is already held on the directory.
func TryExclusiveDirLock(path string) (*FileLock, error) {
	return TryExclusiveLock(path, true)
}

// TryExclusiveLock takes an exclusive lock on a file/directory without blocking.
// It will return ErrLocked if any lock is already held on the file/directory.
func TryExclusiveLock(path string, isDir bool) (*FileLock, error) {
	l, err := NewLock(path, isDir)
	if err != nil {
		return nil, err
	}
	err = l.TryExclusiveLock()
	if err != nil {
		return nil, err
	}
	return l, err
}

// ExclusiveLock takes an exclusive lock.
// This is idempotent when the Lock already represents an exclusive lock,
// and promotes a shared lock to exclusive atomically.
// It will block if an exclusive lock is already held.
func (l *FileLock) ExclusiveLock() error {
	return l.Lock(false, true)
}

// ExclusiveRegFileLock takes an exclusive lock on a file.
// It will block if an exclusive lock is already held on the file.
func ExclusiveRegFileLock(path string) (*FileLock, error) {
	return ExclusiveLock(path, false)
}

// ExclusiveDirLock takes an exclusive lock on a directory.
// It will block if an exclusive lock is already held on the directory.
func ExclusiveDirLock(path string) (*FileLock, error) {
	return ExclusiveLock(path, true)
}

// ExclusiveLock takes an exclusive lock on a file/directory.
// It will block if an exclusive lock is already held on the file/directory.
func ExclusiveLock(path string, isDir bool) (*FileLock, error) {
	l, err := NewLock(path, isDir)
	if err == nil {
		err = l.ExclusiveLock()
	}
	if err != nil {
		return nil, err
	}
	return l, nil
}

// TrySharedLock takes a co-operative (shared) lock without blocking.
// This is idempotent when the Lock already represents a shared lock,
// and tries demote an exclusive lock to shared atomically.
// It will return ErrLocked if an exclusive lock already exists.
func (l *FileLock) TrySharedLock() error {
	return l.Lock(true, false)
}

// TrySharedRegFileLock takes a co-operative (shared) lock on a file without blocking.
// It will return ErrLocked if an exclusive lock already exists on the file.
func TrySharedRegFileLock(path string) (*FileLock, error) {
	return TrySharedLock(path, false)
}

// TrySharedDirLock takes a co-operative (shared) lock on a directory without blocking.
// It will return ErrLocked if an exclusive lock already exists on the directory.
func TrySharedDirLock(path string) (*FileLock, error) {
	return TrySharedLock(path, true)
}

// TrySharedLock takes a co-operative (shared) lock on a file/directory without blocking.
// It will return ErrLocked if an exclusive lock already exists on the file/directory.
func TrySharedLock(path string, isDir bool) (*FileLock, error) {
	l, err := NewLock(path, isDir)
	if err != nil {
		return nil, err
	}
	err = l.TrySharedLock()
	if err != nil {
		return nil, err
	}
	return l, nil
}

// SharedLock takes a co-operative (shared) lock on.
// This is idempotent when the Lock already represents a shared lock,
// and demotes an exclusive lock to shared atomically.
// It will block if an exclusive lock is already held.
func (l *FileLock) SharedLock() error {
	return l.Lock(false, false)
}

// SharedRegFileLock takes a co-operative (shared) lock on a file.
// It will block if an exclusive lock is already held on the file.
func SharedRegFileLock(path string) (*FileLock, error) {
	return TrySharedLock(path, false)
}

// SharedDirLock takes a co-operative (shared) lock on a directory.
// It will block if an exclusive lock is already held on the directory.
func SharedDirLock(path string) (*FileLock, error) {
	return TrySharedLock(path, true)
}

// SharedLock takes a co-operative (shared) lock on a file/directory.
// It will block if an exclusive lock is already held on the file/directory.
func SharedLock(path string, isDir bool) (*FileLock, error) {
	l, err := NewLock(path, isDir)
	if err != nil {
		return nil, err
	}
	err = l.SharedLock()
	if err != nil {
		return nil, err
	}
	return l, nil
}

// Lock takes a lock on a directory. When exclusive is true, an exclusive lock is
// requested, elsewhere a co-operative (shared lock) is requested.
// If try is true it will return ErrLocked if an exclusive (when exclusive is
// false) or any lock (when exclusive is true) is already held on the
// directory.
// If the file is changed between opening and acquiring the lock an
// ErrLockFileChanged is returned.
func (l *FileLock) Lock(try bool, exclusive bool) error {
	var flags int
	if exclusive {
		flags = syscall.LOCK_EX
	} else {
		flags = syscall.LOCK_SH
	}
	if try {
		flags |= syscall.LOCK_NB
	}

	err := syscall.Flock(l.fd, flags)
	if try && err == syscall.EWOULDBLOCK {
		err = ErrLocked
	}
	if err != nil {
		return err
	}

	fd, err := syscall.Open(l.path, syscall.O_RDONLY, 0)
	// If there's an error opening the file return an ErrLockFileChanged
	if err != nil {
		return ErrFileChanged
	}
	var stat syscall.Stat_t
	err = syscall.Fstat(fd, &stat)
	if err != nil {
		return err
	}
	if l.stat.Ino != stat.Ino || l.stat.Dev != stat.Dev {
		return ErrFileChanged
	}
	return nil
}

// Unlock unlocks the lock
func (l *FileLock) Unlock() error {
	return syscall.Flock(l.fd, syscall.LOCK_UN)
}

// Fd returns the lock's file descriptor, or an error if the lock is closed
func (l *FileLock) Fd() (int, error) {
	var err error
	if l.fd == -1 {
		err = errors.New("lock closed")
	}
	return l.fd, err
}

// Close closes the lock which implicitly unlocks it as well
func (l *FileLock) Close() error {
	fd := l.fd
	l.fd = -1
	return syscall.Close(fd)
}

// NewLock opens a new lock on a file without acquisition
func NewLock(path string, isDir bool) (*FileLock, error) {
	l := &FileLock{path: path, fd: -1}

	mode := syscall.O_RDONLY
	if isDir {
		mode |= syscall.O_DIRECTORY
	}
	// we can't use os.OpenFile as Go sets O_CLOEXEC
	lfd, err := syscall.Open(l.path, mode, 0)
	if err != nil {
		if err == syscall.ENOENT {
			err = ErrNotExist
		} else if err == syscall.EACCES {
			err = ErrPermission
		}
		return nil, err
	}
	l.fd = lfd
	err = syscall.Fstat(lfd, &l.stat)
	if err != nil {
		return nil, err
	}
	// Check if the file is a regular file
	if !isDir && !(l.stat.Mode&syscall.S_IFMT == syscall.S_IFREG) {
		return nil, ErrNotRegular
	}

	return l, nil
}

func NewDirLock(path string) (*FileLock, error) {
	return NewLock(path, true)
}

func NewFileLock(path string) (*FileLock, error) {
	return NewLock(path, false)
}
