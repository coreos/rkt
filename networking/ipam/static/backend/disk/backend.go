package disk

import (
	"io/ioutil"
	"net"
	"os"
	"path/filepath"

	"github.com/coreos/rocket/pkg/lock"
)

var defaultDataDir = "/var/lib/rkt/networks"

type Store struct {
	lock.FileLock
	dataDir string
}

func New(network string) (*Store, error) {
	dir := filepath.Join(defaultDataDir, network)
	if err := os.MkdirAll(dir, 0644); err != nil {
		return nil, err
	}

	lk, err := lock.NewDirLock(dir)
	if err != nil {
		return nil, err
	}
	return &Store{*lk, dir}, nil
}

func (s *Store) Lock() error {
	return s.ExclusiveLock()
}

func (s *Store) Reserve(id string, ip net.IP) (bool, error) {
	fname := filepath.Join(s.dataDir, ip.String())
	f, err := os.OpenFile(fname, os.O_RDWR|os.O_EXCL|os.O_CREATE, 0644)
	if os.IsExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if _, err := f.WriteString(id); err != nil {
		f.Close()
		os.Remove(f.Name())
		return false, err
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return false, err
	}
	return true, nil
}

func (s *Store) Release(ip net.IP) error {
	return os.Remove(filepath.Join(s.dataDir, ip.String()))
}

// N.B. This function eats errors to be tolerant and
// release as much as possible
func (s *Store) ReleaseByContainerID(id string) error {
	err := filepath.Walk(s.dataDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		data, err := ioutil.ReadFile(path)
		if err != nil {
			return nil
		}
		if string(data) == id {
			if err := os.Remove(path); err != nil {
				return nil
			}
		}
		return nil
	})
	return err
}
