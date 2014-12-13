package lock

// Package lock implements simple locking primitives on a directory using flock
import (
	"errors"
	"syscall"
)

var (
	ErrLocked = errors.New("directory already locked")
)

type DirLock interface {
	// TryExclusiveLock() attempts to take an exclusive lock on a directory.
	// It will return ErrLocked if any lock is already held on the directory.
	TryExclusiveLock() error
	// TrySharedLock takes a co-operative (shared) lock on a directory.
	// It will return ErrLocked if an exclusive lock already exists on the directory.
	TrySharedLock() error
	// ExclusiveLock takes an exclusive lock on a directory.
	// It will block if any lock is already held on the directory.
	ExclusiveLock() error
	// SharedLock takes a co-operative (shared) lock on a directory.
	// It will block if an exclusive lock is already held on the directory.
	SharedLock() error
	// Unlock() releases a held lock on a directory.
	Unlock() error
	// Close() closes the file representing the lock
	// It will implicitly unlock if locked
	Close() error
	// Fd() returns the fd number for the file representing the lock
	Fd() (int, error)
}

type lock struct {
	dir string
	fd  int
}

func (l *lock) path() string {
	return l.dir
}

// Fd returns the lock's file descriptor
func (l *lock) Fd() (int, error) {
	var err error
	if l.fd == -1 {
		err = errors.New("lock closed")
	}
	return l.fd, err
}

// TryExclusiveLock acquires exclusivity on the lock without blocking
func (l *lock) TryExclusiveLock() error {
	err := syscall.Flock(l.fd, syscall.LOCK_EX|syscall.LOCK_NB)
	if err == syscall.EWOULDBLOCK {
		return ErrLocked
	}
	return err
}

// ExclusiveLock acquires exclusivity on the lock without blocking
func (l *lock) ExclusiveLock() error {
	return syscall.Flock(l.fd, syscall.LOCK_EX)
}

// TrySharedLock cooperatively acquires the lock without blocking
func (l *lock) TrySharedLock() error {
	err := syscall.Flock(l.fd, syscall.LOCK_SH|syscall.LOCK_NB)
	if err == syscall.EWOULDBLOCK {
		return ErrLocked
	}
	return err
}

// SharedLock cooperatively acquires the lock
func (l *lock) SharedLock() error {
	return syscall.Flock(l.fd, syscall.LOCK_SH)
}

// Unlock unlocks the lock but keeps the lock open
func (l *lock) Unlock() error {
	return syscall.Flock(l.fd, syscall.LOCK_UN)
}

// Close closes the lock which implicitly unlocks it as well
func (l *lock) Close() error {
	fd := l.fd
	l.fd = -1
	return syscall.Close(fd)
}

// NewLock opens a new lock on a directory without acquisition
func NewLock(dir string) (DirLock, error) {
	l := &lock{dir: dir, fd: -1}

	// we can't use os.OpenFile as Go sets O_CLOEXEC
	lfd, err := syscall.Open(l.path(), syscall.O_RDONLY|syscall.O_DIRECTORY, 0)
	if err != nil {
		return nil, err
	}
	l.fd = lfd

	return l, nil
}
