// Copyright 2015 The rkt Authors
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

package treestore

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/appc/spec/pkg/acirenderer"
	"github.com/appc/spec/schema/types"
	"github.com/coreos/rkt/common/image/aci"
	"github.com/coreos/rkt/pkg/fileutil"
	"github.com/coreos/rkt/pkg/kvdb"
	"github.com/coreos/rkt/pkg/lock"
	"github.com/coreos/rkt/pkg/sys"
	"github.com/coreos/rkt/pkg/user"
	"github.com/coreos/rkt/store/casref/rwcasref"
	"github.com/coreos/rkt/store/manifestcache"

	"github.com/boltdb/bolt"
	"github.com/hashicorp/errwrap"
)

const (
	defaultPathPerm = os.FileMode(0770 | os.ModeSetgid)
	defaultFilePerm = os.FileMode(0660)

	hashPrefix = "sha256-"
)

// Store represents a store of rendered images
type Store struct {
	dir       string
	renderDir string
	// Path to previous implementaion render directory
	// TODO(sgotti) remove when backward compatibility isn't needed anymore
	oldRenderDir string
	store        *rwcasref.Store
	aciregistry  *aci.ACIRegistry
	db           *kvdb.DB
	lockDir      string
}

func NewStore(dir string, store *rwcasref.Store, aciManifestCache *manifestcache.ACIManifestCache) (*Store, error) {
	// We need to allow the store's setgid bits (if any) to propagate, so
	// disable umask
	um := syscall.Umask(0)
	defer syscall.Umask(um)

	ts := &Store{dir: dir, renderDir: filepath.Join(dir, "tree"), store: store}

	if err := os.MkdirAll(ts.dbDir(), defaultPathPerm); err != nil {
		return nil, errwrap.Wrap(errors.New("cannot create treestore db dir"), err)
	}

	ts.lockDir = filepath.Join(dir, "locks")
	if err := os.MkdirAll(ts.lockDir, 0755); err != nil {
		return nil, errwrap.Wrap(errors.New("cannot create treestore locks dir"), err)
	}

	if err := ts.initDB(); err != nil {
		return nil, errwrap.Wrap(errors.New("cannot initialize treestore db"), err)
	}
	ts.db = kvdb.NewDB(ts.dbFile(), defaultFilePerm)

	ts.aciregistry = aci.NewACIRegistry(store, aciManifestCache)

	return ts, nil
}

func (ts *Store) dbDir() string {
	return filepath.Join(ts.dir, "db")
}

func (ts *Store) dbFile() string {
	return filepath.Join(ts.dbDir(), "db")
}

func (ts *Store) initDB() error {
	db := kvdb.NewDB(ts.dbFile(), defaultFilePerm)

	// Create the "info" bucket
	if err := db.DoRW(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(infobucket))
		return err
	}); err != nil {
		return err
	}
	return nil
}

// SetOldTreeStoreDir sets the old treestore dir, used for backward compatibility
// TODO(sgotti) remove when backward compatibility isn't needed anymore
func (ts *Store) SetOldTreeStoreDir(olddir string) {
	ts.oldRenderDir = olddir
}

// GetInfo returns the treestore info for the specified id
func (ts *Store) GetInfo(id string) (*Info, error) {
	var info *Info
	if err := ts.db.DoRO(func(tx *bolt.Tx) error {
		var err error
		info, err = getInfo(tx, id)
		if err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return info, nil
}

// GetInfosByImageDigest returns all the treestore infos for the specified
// image digest
func (ts *Store) GetInfosByImageDigest(digest string) ([]*Info, error) {
	if digest == "" {
		return nil, fmt.Errorf("empty digest")
	}
	var infos []*Info
	if err := ts.db.DoRO(func(tx *bolt.Tx) error {
		var err error
		infos, err = getInfosByImageDigest(tx, digest)
		if err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return infos, nil
}

// Render renders a treestore for the given image digest if it's not
// already fully rendered.
// Users of treestore should call s.Render before using it to ensure
// that the treestore is completely rendered.
// Returns the id of the rendered treestore
func (ts *Store) Render(digest string, rebuild bool) (id string, err error) {
	// Get the full digest
	digest, err = ts.store.ResolveDigest(digest)
	if err != nil {
		return "", err
	}
	id, err = ts.calculateID(digest)
	if err != nil {
		return "", errwrap.Wrap(errors.New("cannot calculate treestore id"), err)
	}

	// this lock references the treestore dir for the specified id.
	treeStoreKeyLock, err := lock.ExclusiveKeyLock(ts.lockDir, id)
	if err != nil {
		return "", errwrap.Wrap(errors.New("error locking tree store"), err)
	}
	defer treeStoreKeyLock.Close()

	if !rebuild {
		rendered, err := ts.IsRendered(id)
		if err != nil {
			return "", errwrap.Wrap(errors.New("cannot determine if tree is already rendered"), err)
		}
		if rendered {
			return id, nil
		}
	}
	// Firstly remove a possible partial treestore if existing.
	// This is needed as a previous treestore removal operation could have
	// failed cleaning the tree store leaving some stale files.
	if err := ts.remove(id); err != nil {
		return "", err
	}
	if err = ts.render(id, digest); err != nil {
		return "", err
	}

	return id, nil
}

// IsRendered checks if the tree store with the provided id is fully rendered
func (ts *Store) IsRendered(id string) (bool, error) {
	var info *Info
	if err := ts.db.DoRO(func(tx *bolt.Tx) error {
		var err error
		info, err = getInfo(tx, id)
		if err != nil {
			return err
		}
		return nil
	}); err != nil {
		return false, err
	}

	if info == nil {
		return false, nil
	}
	return true, nil
}

// Check verifies the treestore consistency for the specified id.
func (ts *Store) Check(id string) (string, error) {
	treeStoreKeyLock, err := lock.SharedKeyLock(ts.lockDir, id)
	if err != nil {
		return "", errwrap.Wrap(errors.New("error locking tree store"), err)
	}
	defer treeStoreKeyLock.Close()

	return ts.check(id)
}

// Remove removes the rendered image in tree store with the given id.
func (ts *Store) Remove(id string) error {
	// Backward compatibility check for old treestore version
	// If a rootfs for the provided id exists in the old treestore remove it.
	// TODO(sgotti) remove when backward compatibility isn't needed anymore
	if ts.oldPathExists(id) {
		return os.RemoveAll(ts.getOldPath(id))
	}

	treeStoreKeyLock, err := lock.ExclusiveKeyLock(ts.lockDir, id)
	if err != nil {
		return errwrap.Wrap(errors.New("error locking tree store"), err)
	}
	defer treeStoreKeyLock.Close()

	if err := ts.remove(id); err != nil {
		return errwrap.Wrap(errors.New("error removing the tree store"), err)
	}

	return nil
}

// ListIDs returns a slice containing all the fully rendered treestore's IDs
func (ts *Store) ListIDs() ([]string, error) {
	var infos []*Info
	if err := ts.db.DoRO(func(tx *bolt.Tx) error {
		var err error
		infos, err = getAllInfos(tx)
		if err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}

	var treeStoreIDs []string
	for _, i := range infos {
		treeStoreIDs = append(treeStoreIDs, i.ID)

	}

	// Backward compatibility code for old treestore version
	// add also old treestore ids
	// TODO(sgotti) remove when backward compatibility isn't needed anymore
	ls, err := ioutil.ReadDir(ts.oldRenderDir)
	// We want to ignore errors on readdir on the old path
	if err == nil {
		for _, p := range ls {
			if p.IsDir() {
				id := filepath.Base(p.Name())
				// TODO(sgotti) handle duplicated ids between
				// old and new render dirs? It shouldn't
				// happen due to different naming.
				treeStoreIDs = append(treeStoreIDs, id)
			}
		}
	}

	return treeStoreIDs, nil
}

// GetPath returns the absolute path of the treestore for the provided id.
// It doesn't ensure that the path exists and is fully rendered. This should
// be done calling IsRendered()
func (ts *Store) GetPath(id string) string {
	return filepath.Join(ts.renderDir, id)
}

// GetRootFS returns the absolute path of the rootfs for the provided id.
// It doesn't ensure that the rootfs exists and is fully rendered. This should
// be done calling IsRendered()
func (ts *Store) GetRootFS(id string) string {
	// Backward compatibility check for old treestore version
	// If a rootfs for the provided id exists in the old treestore return it.
	// TODO(sgotti) remove when backward compatibility isn't needed anymore
	if ts.oldPathExists(id) {
		return filepath.Join(ts.getOldPath(id), "rootfs")
	}

	return filepath.Join(ts.GetPath(id), "rootfs")
}

// calculateID calculates the treestore ID for the given image digest.
// For ACI image types the ID is computed as an sha256 hash of the flattened
// dependency tree image digests. In this way the ID may change for the same
// digest if the image's dependencies change.
// For future OCI image types the ID will just be the digest (in this case the
// manifest digest)
func (ts *Store) calculateID(digest string) (string, error) {
	hash, err := types.NewHash(digest)
	if err != nil {
		return "", err
	}
	images, err := acirenderer.CreateDepListFromImageID(*hash, ts.aciregistry)
	if err != nil {
		return "", err
	}

	var digests []string
	for _, image := range images {
		digests = append(digests, image.Key)
	}
	imagesString := strings.Join(digests, ",")
	h := sha256.New()
	h.Write([]byte(imagesString))
	return "deps-" + hashString(h), nil
}

// render renders the image with the provided digest in the treestore. id references
// that specific tree store rendered image.
// render, to avoid having a rendered image with old stale files, requires that
// the destination directory doesn't exist (usually remove should be called
// before render)
func (ts *Store) render(id string, digest string) error {
	treepath := ts.GetPath(id)
	fi, _ := os.Stat(treepath)
	if fi != nil {
		return fmt.Errorf("path %s already exists", treepath)
	}
	imageID, err := types.NewHash(digest)
	if err != nil {
		return errwrap.Wrap(errors.New("cannot convert digest to imageID"), err)
	}
	if err := os.MkdirAll(treepath, 0755); err != nil {
		return errwrap.Wrap(fmt.Errorf("cannot create treestore directory %s", treepath), err)
	}
	err = aci.RenderACIWithImageID(*imageID, treepath, ts.aciregistry, user.NewBlankUidRange())
	if err != nil {
		return errwrap.Wrap(errors.New("cannot render aci"), err)
	}
	checksum, err := ts.checksum(id)
	if err != nil {
		return errwrap.Wrap(errors.New("cannot calculate tree checksum"), err)
	}
	// before writing the treestore info in the db we need to ensure that all data is fsynced
	dfd, err := syscall.Open(treepath, syscall.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer syscall.Close(dfd)
	if err := sys.Syncfs(dfd); err != nil {
		return errwrap.Wrap(errors.New("failed to sync data"), err)
	}
	if err := syscall.Fsync(dfd); err != nil {
		return errwrap.Wrap(errors.New("failed to sync tree store directory"), err)
	}

	size, err := ts.size(id)
	if err != nil {
		return err
	}

	info := &Info{ID: id, ImageDigest: digest, Checksum: checksum, Size: size}
	if err := ts.db.DoRW(func(tx *bolt.Tx) error {
		return writeInfo(tx, info)
	}); err != nil {
		return err
	}

	return nil
}

// remove remove the treestore info from the db and clean the directory for the
// provided id
func (ts *Store) remove(id string) error {
	treepath := ts.GetPath(id)
	// If tree path doesn't exist we're done
	_, err := os.Stat(treepath)
	if err != nil && os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return errwrap.Wrap(errors.New("failed to open tree store directory"), err)
	}

	if err := ts.db.DoRW(func(tx *bolt.Tx) error {
		return removeInfo(tx, id)
	}); err != nil {
		return err
	}

	if err := os.RemoveAll(treepath); err != nil {
		return err
	}
	return nil
}

// getOldPath returns the absolute path of the treestore for the provided id.
// It doesn't ensure that the path exists and is fully rendered. This should
// be done calling IsRendered()
func (ts *Store) getOldPath(id string) string {
	return filepath.Join(ts.oldRenderDir, id)
}

func (ts *Store) oldPathExists(id string) bool {
	if ts.oldRenderDir != "" {
		_, err := os.Stat(ts.getOldPath(id))
		// We want to ignore errors on stat on old path so just take
		// any non error as the path exists (without checkong os.IsNotExists
		if err == nil {
			return true
		}
	}
	return false
}

// checksum calculates a checksum of the rendered image. It uses the same
// functions used to create a tar but instead of writing the full archive is
// just computes the sha256 sum of the file infos and contents.
func (ts *Store) checksum(id string) (string, error) {
	treepath := ts.GetPath(id)

	hash := sha256.New()
	iw := newHashWriter(hash)
	err := filepath.Walk(treepath, buildWalker(treepath, iw))
	if err != nil {
		return "", errwrap.Wrap(errors.New("error walking rootfs"), err)
	}

	checksum := hashString(hash)

	return checksum, nil
}

// check calculates the actual rendered image's checksum and verifies that it matches
// the saved value. Returns the calculated checksum.
func (ts *Store) check(id string) (string, error) {
	var info *Info
	if err := ts.db.DoRO(func(tx *bolt.Tx) error {
		var err error
		info, err = getInfo(tx, id)
		if err != nil {
			return err
		}
		return nil
	}); err != nil {
		return "", err
	}

	if info == nil {
		return "", fmt.Errorf("tree store does not exists")
	}

	curChecksum, err := ts.checksum(id)
	if err != nil {
		return "", errwrap.Wrap(errors.New("cannot calculate tree checksum"), err)
	}
	if curChecksum != info.Checksum {
		return "", fmt.Errorf("wrong tree checksum: %s, expected: %s", curChecksum, info.Checksum)
	}
	return curChecksum, nil
}

// size returns the size of the rootfs for the provided id. It is a relatively
// expensive operation, it goes through all the files and adds up their size.
func (ts *Store) size(id string) (int64, error) {
	sz, err := fileutil.DirSize(ts.GetPath(id))
	if err != nil {
		return -1, errwrap.Wrap(errors.New("error calculating size"), err)
	}
	return sz, nil
}
