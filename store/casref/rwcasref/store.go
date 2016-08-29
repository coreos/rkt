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

package rwcasref

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/boltdb/bolt"
	"github.com/coreos/rkt/pkg/digest"
	"github.com/coreos/rkt/pkg/kvdb"
	"github.com/coreos/rkt/pkg/lock"

	"github.com/hashicorp/errwrap"
	"github.com/peterbourgon/diskv"
)

const (
	defaultPathPerm = os.FileMode(0770 | os.ModeSetgid)
	defaultFilePerm = os.FileMode(0660)
)

var (
	ErrDigestNotFound = errors.New("digest not found")
	ErrRefNotFound    = errors.New("ref not found")
	ErrStaleData      = errors.New("some stale data has been left on disk")
)

type Store struct {
	dir         string
	blobDiskv   *diskv.Diskv
	blobDB      *kvdb.DB
	blobLockDir string
	refDB       *kvdb.DB
}

func NewStore(dir string) (*Store, error) {
	// We need to allow the store's setgid bits (if any) to propagate, so
	// disable umask
	um := syscall.Umask(0)
	defer syscall.Umask(um)

	s := &Store{
		dir: dir,
	}

	s.blobLockDir = filepath.Join(dir, "bloblocks")
	err := os.MkdirAll(s.blobLockDir, defaultPathPerm)
	if err != nil {
		return nil, err
	}

	s.blobDiskv = diskv.New(diskv.Options{
		PathPerm:  defaultPathPerm,
		FilePerm:  defaultFilePerm,
		BasePath:  filepath.Join(dir, "blobs"),
		Transform: blockTransform,
	})

	if err := os.MkdirAll(s.blobDBDir(), defaultPathPerm); err != nil {
		return nil, errwrap.Wrap(errors.New("cannot create treestore db dir"), err)
	}
	if err := s.initBlobDB(); err != nil {
		return nil, errwrap.Wrap(errors.New("cannot initialize treestore db"), err)
	}
	s.blobDB = kvdb.NewDB(s.blobDBFile(), defaultFilePerm)

	if err := os.MkdirAll(s.refDBDir(), defaultPathPerm); err != nil {
		return nil, errwrap.Wrap(errors.New("cannot create treestore db dir"), err)
	}
	if err := s.initRefDB(); err != nil {
		return nil, errwrap.Wrap(errors.New("cannot initialize treestore db"), err)
	}
	s.refDB = kvdb.NewDB(s.refDBFile(), defaultFilePerm)

	return s, nil
}

// Close closes a Store opened with NewStore().
func (s *Store) Close() error {
	return nil
}

func (s *Store) blobDBDir() string {
	return filepath.Join(s.dir, "blobdb")
}

func (s *Store) blobDBFile() string {
	return filepath.Join(s.blobDBDir(), "db")
}

func (s *Store) initBlobDB() error {
	dbFile := s.blobDBFile()
	db := kvdb.NewDB(dbFile, defaultFilePerm)

	// Create the "blobinfo" and "blobdata" buckets
	if err := db.DoRW(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists([]byte(blobinfobucket)); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists([]byte(blobdatabucket)); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func (s *Store) refDBDir() string {
	return filepath.Join(s.dir, "refdb")
}

func (s *Store) refDBFile() string {
	return filepath.Join(s.refDBDir(), "db")
}

func (s *Store) initRefDB() error {
	dbFile := s.refDBFile()
	db := kvdb.NewDB(dbFile, defaultFilePerm)

	// Create the "info" bucket
	if err := db.DoRW(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(refbucket))
		return err
	}); err != nil {
		return err
	}
	return nil
}

// TODO(sgotti), unexport this and provide other functions for external users
// TmpFile returns an *os.File local to the same filesystem as the Store, or
// any error encountered
func (s *Store) TmpFile() (*os.File, error) {
	dir, err := s.TmpDir()
	if err != nil {
		return nil, err
	}
	return ioutil.TempFile(dir, "")
}

// TODO(sgotti), unexport this and provide other functions for external users
// TmpNamedFile returns an *os.File with the specified name local to the same
// filesystem as the Store, or any error encountered. If the file already
// exists it will return the existing file in read/write mode with the cursor
// at the end of the file.
func (s Store) TmpNamedFile(name string) (*os.File, error) {
	dir, err := s.TmpDir()
	if err != nil {
		return nil, err
	}
	fname := filepath.Join(dir, name)
	_, err = os.Stat(fname)
	if os.IsNotExist(err) {
		return os.Create(fname)
	}
	if err != nil {
		return nil, err
	}
	return os.OpenFile(fname, os.O_RDWR|os.O_APPEND, 0644)
}

// TODO(sgotti), unexport this and provide other functions for external users
// TmpDir creates and returns dir local to the same filesystem as the Store,
// or any error encountered
func (s *Store) TmpDir() (string, error) {
	dir := filepath.Join(s.dir, "tmp")
	if err := os.MkdirAll(dir, defaultPathPerm); err != nil {
		return "", err
	}
	return dir, nil
}

// ResolveDigest resolves a partial digest (of format `sha512-0c45e8c0ab2`) to a full
// digest by considering the digest a prefix and using the store for resolution.
func (s *Store) ResolveDigest(inDigest string) (string, error) {
	digest, a, err := digest.ParseDigest(inDigest)
	if err != nil {
		return "", fmt.Errorf("cannot parse digest %s: %v", inDigest, err)
	}
	// at least prefix-aa (sha256-aa)
	if len(digest) < a.PrefixLen()+3 { // -aa
		return "", fmt.Errorf("digest %q too short", inDigest)
	}

	var blobInfos []*BlobInfo
	err = s.blobDB.DoRO(func(tx *bolt.Tx) error {
		var err error
		blobInfos, err = getBlobInfosWithDigestPrefix(tx, digest)
		return err
	})
	if err != nil {
		return "", errwrap.Wrap(errors.New("error retrieving blob Infos"), err)
	}

	blobCount := len(blobInfos)
	if blobCount == 0 {
		return "", ErrDigestNotFound
	}
	if blobCount != 1 {
		return "", fmt.Errorf("ambiguous digest: %q", inDigest)
	}
	return blobInfos[0].Digest, nil
}

func (s *Store) ReadBlob(digest string) (io.ReadCloser, error) {
	digest, err := s.ResolveDigest(digest)
	if err != nil {
		return nil, errwrap.Wrap(errors.New("error resolving digest"), err)
	}
	blobLock, err := lock.SharedKeyLock(s.blobLockDir, digest)
	if err != nil {
		return nil, errwrap.Wrap(errors.New("error locking blob"), err)
	}
	defer blobLock.Close()

	err = s.blobDB.DoRO(func(tx *bolt.Tx) error {
		blobinfo, err := getBlobInfo(tx, digest)
		if err != nil {
			return errwrap.Wrap(errors.New("error getting blob info"), err)
		}
		if blobinfo == nil {
			return fmt.Errorf("cannot find blob with digest: %s", digest)
		}
		return nil
	})
	if err != nil {
		return nil, errwrap.Wrap(fmt.Errorf("cannot get blob info for %q from db", digest), err)
	}

	return s.blobDiskv.ReadStream(digest, false)
}

func (s *Store) WriteBlob(r io.Reader, mediaType string, blobData map[string][]byte, a digest.Algorithm) (string, error) {
	// We need to allow the store's setgid bits (if any) to propagate, so
	// disable umask
	um := syscall.Umask(0)
	defer syscall.Umask(um)

	// Write the decompressed image (tar) to a temporary file on disk, and
	// tee so we can generate the hash
	d := digest.NewDigester(a)
	h := d.Hash()
	tr := io.TeeReader(r, h)
	fh, err := s.TmpFile()
	if err != nil {
		return "", errwrap.Wrap(errors.New("error creating image"), err)
	}
	sz, err := io.Copy(fh, tr)
	if err != nil {
		return "", errwrap.Wrap(errors.New("error copying image"), err)
	}
	if err := fh.Close(); err != nil {
		return "", errwrap.Wrap(errors.New("error closing image"), err)
	}

	digest := d.Digest()
	blobLock, err := lock.ExclusiveKeyLock(s.blobLockDir, digest)
	if err != nil {
		return "", errwrap.Wrap(errors.New("error locking image"), err)
	}
	defer blobLock.Close()

	if err = s.blobDiskv.Import(fh.Name(), digest, true); err != nil {
		return "", errwrap.Wrap(errors.New("error importing image"), err)
	}

	// Save blobinfo
	if err = s.blobDB.DoRW(func(tx *bolt.Tx) error {
		blobInfo := &BlobInfo{
			Digest:     digest,
			MediaType:  mediaType,
			ImportTime: time.Now(),
			Size:       sz,
		}
		if err := writeBlobInfo(tx, blobInfo); err != nil {
			return err
		}
		for dataType, data := range blobData {
			blobData := &BlobData{
				Digest:   digest,
				DataType: dataType,
				Data:     data,
			}
			if err := writeBlobData(tx, blobData); err != nil {
				return err
			}
		}
		return writeBlobInfo(tx, blobInfo)
	}); err != nil {
		return "", errwrap.Wrap(errors.New("error writing blob info"), err)
	}

	return digest, nil
}

func (s *Store) SetBlobData(digest string, dataType string, data []byte) error {
	if dataType == "" {
		return errors.New("empty datatype")
	}
	digest, err := s.ResolveDigest(digest)
	if err != nil {
		return errwrap.Wrap(errors.New("error resolving digest"), err)
	}
	blobKeyLock, err := lock.ExclusiveKeyLock(s.blobLockDir, digest)
	if err != nil {
		return errwrap.Wrap(errors.New("error locking image"), err)
	}
	defer blobKeyLock.Close()

	if err = s.blobDB.DoRW(func(tx *bolt.Tx) error {
		blobData := &BlobData{
			Digest:   digest,
			DataType: dataType,
			Data:     data,
		}
		return writeBlobData(tx, blobData)
	}); err != nil {
		return errwrap.Wrap(errors.New("error writing blob data"), err)
	}
	return nil
}

func (s *Store) GetBlobData(digest string, dataType string) ([]byte, error) {
	if dataType == "" {
		return nil, errors.New("empty datatype")
	}
	digest, err := s.ResolveDigest(digest)
	if err != nil {
		return nil, errwrap.Wrap(errors.New("error resolving digest"), err)
	}
	blobKeyLock, err := lock.SharedKeyLock(s.blobLockDir, digest)
	if err != nil {
		return nil, errwrap.Wrap(errors.New("error locking image"), err)
	}
	defer blobKeyLock.Close()

	var blobData *BlobData
	if err = s.blobDB.DoRO(func(tx *bolt.Tx) error {
		var err error
		blobData, err = getBlobData(tx, digest, dataType)
		if err != nil {
			return errwrap.Wrap(errors.New("error getting blobdata"), err)
		}
		return nil
	}); err != nil {
		return nil, errwrap.Wrap(errors.New("error getting blobinfo"), err)
	}
	return blobData.Data, nil
}

// RemoveBlob removes the blob and all its data with the given digest.
// If force is false it'll fail if a blob as some references on it. If force is
// true it'll remove the blob and all its references.
// If some error occurs removing some non transactional data a blob is
// considered as removed but ErrStaleData is returned.
func (s *Store) RemoveBlob(digest string, force bool) error {
	digest, err := s.ResolveDigest(digest)
	if err != nil {
		return errwrap.Wrap(errors.New("error resolving digest"), err)
	}
	blobKeyLock, err := lock.ExclusiveKeyLock(s.blobLockDir, digest)
	if err != nil {
		return errwrap.Wrap(errors.New("error locking image"), err)
	}
	defer blobKeyLock.Close()

	refs := []*Ref{}
	err = s.refDB.DoRO(func(tx *bolt.Tx) error {
		var err error
		refs, err = getRefsByDigest(tx, digest)
		if err != nil {
			return errwrap.Wrap(errors.New("error getting refs"), err)
		}
		return nil
	})
	if err != nil {
		return errwrap.Wrap(fmt.Errorf("cannot get refs for digest: %s from db", digest), err)
	}
	if len(refs) != 0 {
		if !force {
			return errors.New("blob is referenced")
		} else {
			err = s.refDB.DoRW(func(tx *bolt.Tx) error {
				for _, ref := range refs {
					if err := removeRef(tx, ref.ID); err != nil {
						return errwrap.Wrap(errors.New("error removing ref"), err)
					}
				}
				return nil
			})
			if err != nil {
				return errwrap.Wrap(fmt.Errorf("cannot get refs for digest: %s from db", digest), err)
			}
		}
	}

	// Then remove blobinfo from the db
	err = s.blobDB.DoRW(func(tx *bolt.Tx) error {
		blobInfo, err := getBlobInfo(tx, digest)
		if err != nil {
			return errwrap.Wrap(errors.New("error getting blobinfo"), err)
		}
		if blobInfo == nil {
			return fmt.Errorf("cannot find blob with digest: %s", digest)
		}

		if err := removeBlobInfo(tx, digest); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return errwrap.Wrap(fmt.Errorf("cannot remove blob with digest: %s from db", digest), err)
	}

	// Then remove non transactional entries from the blob
	if err := s.blobDiskv.Erase(digest); err != nil {
		return errwrap.Wrap(ErrStaleData, errwrap.Wrap(fmt.Errorf("cannot remove blob with digest: %s from disk store", digest), err))
	}
	return nil
}

// GetRef return the digest associated with the ref. If the ref doesn't exist
// ErrRefNotFound is returned.
func (s *Store) GetRef(id string) (string, error) {
	var ref *Ref
	if err := s.refDB.DoRO(func(tx *bolt.Tx) error {
		var err error
		ref, err = getRef(tx, id)
		if err != nil {
			return err
		}
		return nil
	}); err != nil {
		return "", fmt.Errorf("error getting ref: %v", err)
	}
	if ref == nil {
		return "", ErrRefNotFound
	}
	return ref.Digest, nil
}

// SetRef sets the reference to an image digest
// It's up to the caller to check that all the required blobs for a specific
// image type are in the store (for example in OCI all the required blobs
// should be available). SetRef only checks that the referenced blob exists.
func (s *Store) SetRef(id string, digest string) error {
	digest, err := s.ResolveDigest(digest)
	if err != nil {
		return errwrap.Wrap(errors.New("error resolving digest"), err)
	}
	// Take a blobKeyLock to avoid blob removal between checking that it
	// exists and setting the ref
	blobKeyLock, err := lock.ExclusiveKeyLock(s.blobLockDir, digest)
	if err != nil {
		return errwrap.Wrap(errors.New("error locking image"), err)
	}
	defer blobKeyLock.Close()

	ok, err := s.HasBlob(digest)
	if err != nil {
		return errwrap.Wrap(errors.New("error checking blob existance"), err)
	}
	if !ok {
		return fmt.Errorf("cannot set reference to unexistant blob %q", digest)
	}
	ref := &Ref{
		ID:     id,
		Digest: digest,
	}
	if err := s.refDB.DoRW(func(tx *bolt.Tx) error {
		return writeRef(tx, ref)
	}); err != nil {
		return fmt.Errorf("error writing ref: %v", err)
	}
	return nil
}

func (s *Store) RemoveRef(id string) error {
	if err := s.refDB.DoRW(func(tx *bolt.Tx) error {
		return removeRef(tx, id)
	}); err != nil {
		return fmt.Errorf("error removing ref: %v", err)
	}
	return nil
}

func (s *Store) GetAllRefs() ([]*Ref, error) {
	var refs []*Ref
	if err := s.refDB.DoRO(func(tx *bolt.Tx) error {
		var err error
		refs, err = getAllRefs(tx)
		if err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("error getting refs: %v", err)
	}
	return refs, nil
}

func (s *Store) GetBlobInfo(digest string) (*BlobInfo, error) {
	var blobInfo *BlobInfo
	err := s.blobDB.DoRO(func(tx *bolt.Tx) error {
		var err error
		blobInfo, err = getBlobInfo(tx, digest)
		return err
	})
	if err != nil {
		return nil, err
	}
	return blobInfo, nil
}

func (s *Store) GetBlobsInfosByMediaType(mediaType string) ([]*BlobInfo, error) {
	var blobInfos []*BlobInfo
	err := s.blobDB.DoRO(func(tx *bolt.Tx) error {
		var err error
		blobInfos, err = getBlobInfosByMediaType(tx, mediaType)
		return err
	})
	if err != nil {
		return nil, err
	}
	return blobInfos, nil
}

func (s *Store) HasBlob(digest string) (bool, error) {
	var blobInfo *BlobInfo
	err := s.blobDB.DoRO(func(tx *bolt.Tx) error {
		var err error
		blobInfo, err = getBlobInfo(tx, digest)
		return err
	})
	if err != nil {
		return false, err
	}
	return blobInfo != nil, nil
}
