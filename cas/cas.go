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

package cas

import (
	"bufio"
	"bytes"
	"crypto/sha512"
	"database/sql"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/appc/spec/aci"
	"github.com/appc/spec/schema"
	"github.com/appc/spec/schema/types"

	"github.com/coreos/rocket/Godeps/_workspace/src/github.com/peterbourgon/diskv"
)

// TODO(philips): use a database for the secondary indexes like remoteType and
// appType. This is OK for now though.
const (
	blobType int64 = iota
	imageManifestType

	defaultPathPerm os.FileMode = 0777

	// To ameliorate excessively long paths, keys for the (blob)store use
	// only the first half of a sha512 rather than the entire sum
	hashPrefix = "sha512-"
	lenHash    = sha512.Size       // raw byte size
	lenHashKey = (lenHash / 2) * 2 // half length, in hex characters
	lenKey     = len(hashPrefix) + lenHashKey
)

var diskvStores = [...]string{
	"blob",
	"imageManifest",
}

// Store encapsulates a content-addressable-storage for storing ACIs on disk.
type Store struct {
	base   string
	stores []*diskv.Diskv
	db     *DB
}

var dbCreateStmts = [...]string{
	// remote
	"CREATE TABLE IF NOT EXISTS remote (aciurl string, sigurl string, etag string, blobkey string);",
	"CREATE UNIQUE INDEX IF NOT EXISTS aciurlidx ON remote (aciurl)",
	// aciinfo, blobkey is the primary key and it matches the key used to save that aci in the blob store
	"CREATE TABLE IF NOT EXISTS aciinfo (blobkey string, appname string, importtime time, latest bool);",
	"CREATE UNIQUE INDEX IF NOT EXISTS blobkeyidx ON aciinfo (blobkey)",
}

func NewStore(base string) (*Store, error) {
	ds := &Store{
		base:   base,
		stores: make([]*diskv.Diskv, len(diskvStores)),
	}

	for i, p := range diskvStores {
		ds.stores[i] = diskv.New(diskv.Options{
			BasePath:  filepath.Join(base, "cas", p),
			Transform: blockTransform,
		})
	}
	db, err := NewDB(filepath.Join(base, "cas", "db"))
	if err != nil {
		return nil, err
	}
	ds.db = db

	// Execute db create statements, this is done everytime NewStore is
	// called so they should use the "IF NOT EXISTS" statements.
	fn := func(tx *sql.Tx) error {
		for _, stmt := range dbCreateStmts {
			_, err = tx.Exec(stmt)
			if err != nil {
				return err
			}
		}
		return nil
	}

	if err = db.Do(fn); err != nil {
		return nil, err
	}

	return ds, nil
}

func (ds Store) tmpFile() (*os.File, error) {
	dir := filepath.Join(ds.base, "tmp")
	if err := os.MkdirAll(dir, defaultPathPerm); err != nil {
		return nil, err
	}
	return ioutil.TempFile(dir, "")
}

// ResolveKey resolves a partial key (of format `sha512-0c45e8c0ab2`) to a full
// key by considering the key a prefix and using the store for resolution.
// If the key is longer than the full key length, it is first truncated.
func (ds Store) ResolveKey(key string) (string, error) {
	if len(key) > lenKey {
		key = key[:lenKey]
	}

	cancel := make(chan struct{})
	var k string
	keyCount := 0
	for k = range ds.stores[blobType].KeysPrefix(key, cancel) {
		keyCount++
		if keyCount > 1 {
			close(cancel)
			break
		}
	}
	if keyCount == 0 {
		return "", fmt.Errorf("no keys found")
	}
	if keyCount != 1 {
		return "", fmt.Errorf("ambiguous key: %q", key)
	}
	return k, nil
}

func (ds Store) ReadStream(key string) (io.ReadCloser, error) {
	return ds.stores[blobType].ReadStream(key, false)
}

func (ds Store) WriteStream(key string, r io.Reader) error {
	return ds.stores[blobType].WriteStream(key, r, true)
}

// WriteACI takes an ACI encapsulated in an io.Reader, decompresses it if
// necessary, and then stores it in the store under a key based on the image ID
// (i.e. the hash of the uncompressed ACI)
// latest defines if the aci has to be marked as the latest (eg. an ACI
// downloaded with the latest pattern: without asking for a specific version)
func (ds Store) WriteACI(r io.Reader, latest bool) (string, error) {
	// Peek at the first 512 bytes of the reader to detect filetype
	br := bufio.NewReaderSize(r, 512)
	hd, err := br.Peek(512)
	switch err {
	case nil:
	case io.EOF: // We may have still peeked enough to guess some types, so fall through
	default:
		return "", fmt.Errorf("error reading image header: %v", err)
	}
	typ, err := aci.DetectFileType(bytes.NewBuffer(hd))
	if err != nil {
		return "", fmt.Errorf("error detecting image type: %v", err)
	}
	dr, err := decompress(br, typ)
	if err != nil {
		return "", fmt.Errorf("error decompressing image: %v", err)
	}

	// Write the decompressed image (tar) to a temporary file on disk, and
	// tee so we can generate the hash
	h := sha512.New()
	tr := io.TeeReader(dr, h)
	fh, err := ds.tmpFile()
	if err != nil {
		return "", fmt.Errorf("error creating image: %v", err)
	}
	if _, err := io.Copy(fh, tr); err != nil {
		return "", fmt.Errorf("error copying image: %v", err)
	}
	im, err := aci.ManifestFromImage(fh)
	if err != nil {
		return "", fmt.Errorf("error extracting ImageManifest: %v", err)
	}
	if err := fh.Close(); err != nil {
		return "", fmt.Errorf("error closing image: %v", err)
	}

	// Import the uncompressed image into the store at the real key
	key := HashToKey(h)
	if err = ds.stores[blobType].Import(fh.Name(), key, true); err != nil {
		return "", fmt.Errorf("error importing image: %v", err)
	}

	// Save the imagemanifest using the same key used for the image
	imj, err := json.Marshal(im)
	if err != nil {
		return "", fmt.Errorf("error marshalling image manifest: %v", err)
	}
	if err = ds.stores[imageManifestType].Write(key, imj); err != nil {
		return "", fmt.Errorf("error importing image: %v", err)
	}

	// Save aciinfo
	ds.db.Do(func(tx *sql.Tx) error {
		aciinfo := &ACIInfo{
			BlobKey:    key,
			AppName:    im.Name.String(),
			ImportTime: time.Now(),
			Latest:     latest,
		}
		return WriteACIInfo(tx, aciinfo)
	})
	return key, nil
}

// GetRemote tries to retrieve a remote with the given ACIURL. found will be
// false if not remote exists
func (ds Store) GetRemote(aciURL string, sigURL string) (*Remote, bool, error) {
	var remote *Remote
	found := false
	err := ds.db.Do(func(tx *sql.Tx) error {
		var err error
		remote, found, err = GetRemote(tx, aciURL, sigURL)
		return err
	})
	return remote, found, err
}

// WriteRemote adds or updates the provided Remote.
func (ds Store) WriteRemote(remote *Remote) error {
	err := ds.db.Do(func(tx *sql.Tx) error {
		return WriteRemote(tx, remote)
	})
	return err
}

// Get the ImageManifest with the specified key.
func (ds Store) GetImageManifest(key string) (*schema.ImageManifest, error) {
	imj, err := ds.stores[imageManifestType].Read(key)
	if err != nil {
		return nil, fmt.Errorf("error importing image: %v", err)
	}
	var im *schema.ImageManifest
	if err = json.Unmarshal(imj, &im); err != nil {
		return nil, fmt.Errorf("error unmashalling imagemanifest: %v", err)
	}
	return im, nil
}

// Get the best ACI that matches app name and the provided labels. It returns
// the blob store key of the given ACI.
// If there are multiple matching ACIs choose the latest one (defined as the
// last one imported in the store).
// If no version label is requested, ACIs marked as latest in the ACIInfo are
// preferred.
func (ds Store) GetACI(name types.ACName, labels types.Labels) (string, error) {
	var curaciinfo *ACIInfo
	versionRequested := false
	if _, ok := labels.Get("version"); ok {
		versionRequested = true
	}

	var aciinfos []*ACIInfo
	err := ds.db.Do(func(tx *sql.Tx) error {
		var err error
		aciinfos, _, err = GetACIInfosWithAppName(tx, name.String())
		return err
	})
	if err != nil {
		return "", err
	}

nextKey:
	for _, aciinfo := range aciinfos {
		im, err := ds.GetImageManifest(aciinfo.BlobKey)
		if err != nil {
			return "", fmt.Errorf("error getting image manifest: %v", err)
		}

		// The image manifest must have all the requested labels
		for _, l := range labels {
			ok := false
			for _, rl := range im.Labels {
				if l.Name == rl.Name && l.Value == rl.Value {
					ok = true
					break
				}
			}
			if !ok {
				continue nextKey
			}
		}

		if curaciinfo != nil {
			// If no version is requested prefer the acis marked as latest
			if !versionRequested {
				if !curaciinfo.Latest && aciinfo.Latest {
					curaciinfo = aciinfo
					continue nextKey
				}
				if curaciinfo.Latest && !aciinfo.Latest {
					continue nextKey
				}
			}
			// If multiple matching image manifests are found, choose the latest imported in the cas.
			if aciinfo.ImportTime.After(curaciinfo.ImportTime) {
				curaciinfo = aciinfo
			}
		} else {
			curaciinfo = aciinfo
		}
	}

	if curaciinfo != nil {
		return curaciinfo.BlobKey, nil
	}
	return "", fmt.Errorf("aci not found")
}

func (ds Store) Dump(hex bool) {
	for _, s := range ds.stores {
		var keyCount int
		for key := range s.Keys(nil) {
			val, err := s.Read(key)
			if err != nil {
				panic(fmt.Sprintf("key %s had no value", key))
			}
			if len(val) > 128 {
				val = val[:128]
			}
			out := string(val)
			if hex {
				out = fmt.Sprintf("%x", val)
			}
			fmt.Printf("%s/%s: %s\n", s.BasePath, key, out)
			keyCount++
		}
		fmt.Printf("%d total keys\n", keyCount)
	}
}

// HashToKey takes a hash.Hash (which currently _MUST_ represent a full SHA512),
// calculates its sum, and returns a string which should be used as the key to
// store the data matching the hash.
func HashToKey(h hash.Hash) string {
	s := h.Sum(nil)
	return keyToString(s)
}

// keyToString takes a key and returns a shortened and prefixed hexadecimal string version
func keyToString(k []byte) string {
	if len(k) != lenHash {
		panic(fmt.Sprintf("bad hash passed to hashToKey: %x", k))
	}
	return fmt.Sprintf("%s%x", hashPrefix, k)[0:lenKey]
}
