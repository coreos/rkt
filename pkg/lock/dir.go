package lock

// Package lock implements simple locking primitives on a directory using flock
import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

var (
	ErrLocked = errors.New("directory already locked")
)

const (
	errLocked = "resource temporarily unavailable"
)

type DirLock interface {
	// ExclusiveLock() attempts to take an exclusive lock on a directory.
	// It will return ErrLocked if a lock already exists on the directory.
	ExclusiveLock() error
	// SharedLock takes a co-operative (shared) lock on a directory.
	SharedLock() error
}

type lock struct {
	dir string
}

func (l *lock) path() string {
	return filepath.Join(l.dir, ".lock")
}

func (l *lock) getfd() (int, error) {
	// we can't use os.OpenFile as Go sets O_CLOEXEC
	return syscall.Open(l.path(), os.O_WRONLY|os.O_CREATE, uint32(0755))
}

func (l *lock) ExclusiveLock() error {
	fd, err := l.getfd()
	if err != nil {
		return err
	}
	err = syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil && err.Error() == errLocked {
		return ErrLocked
	}
	return err
}

func (l *lock) SharedLock() error {
	fd, err := l.getfd()
	if err != nil {
		return err
	}
	return syscall.Flock(fd, syscall.LOCK_SH|syscall.LOCK_NB)
}

func NewLock(dir string) (DirLock, error) {
	fi, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		return nil, errors.New("must be directory")
	}
	return &lock{dir}, nil
}
