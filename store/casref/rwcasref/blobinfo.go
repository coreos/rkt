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
	"bytes"
	"encoding/json"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/boltdb/bolt"
)

const (
	blobInfoDigest    = "digest"
	blobInfoMediaType = "mediatype"
)

var (
	blobinfobucket = []byte("blobinfo")
)

// BlobInfo contains the treestore information.
type BlobInfo struct {
	// Blob digest
	Digest string
	// MediaType represent the type of blob contents
	MediaType string
	// Render image size
	Size int64
	// Time the blob was imported in the store
	ImportTime time.Time
}

func checkKeyPart(k string) {
	if strings.Contains(k, "/") {
		panic(fmt.Errorf("bad key value %q", k))
	}
}

func blobInfoDigestKey(digest string) []byte {
	checkKeyPart(digest)
	return []byte(blobInfoDigest + "/" + digest)
}

func blobInfoMediaTypeDigestKey(mediaType string, digest string) []byte {
	checkKeyPart(string(mediaType))
	checkKeyPart(digest)
	if mediaType == "" {
		panic("empty image value")
	}
	// don't use path.Join since we want to keep the / if digest is empty
	return []byte(blobInfoMediaType + "/" + string(mediaType) + "/" + digest)
}

func writeBlobInfo(tx *bolt.Tx, bi *BlobInfo) error {
	b := tx.Bucket(blobinfobucket)
	if b == nil {
		return fmt.Errorf("bucket does not exists")
	}
	bij, err := json.Marshal(bi)
	if err != nil {
		return err
	}

	// Insert the json blobinfo using Digest as primary key
	b.Put(blobInfoDigestKey(bi.Digest), bij)

	// Add additional non unique index on mediaType, use "/" as
	// separator between MediaType and Digest
	b.Put(blobInfoMediaTypeDigestKey(bi.MediaType, bi.Digest), nil)

	return nil
}

func getBlobInfo(tx *bolt.Tx, digest string) (*BlobInfo, error) {
	b := tx.Bucket(blobinfobucket)
	if b == nil {
		return nil, fmt.Errorf("bucket does not exists")
	}

	bij := b.Get(blobInfoDigestKey(digest))
	if bij == nil {
		return nil, nil
	}

	var bi *BlobInfo
	if err := json.Unmarshal(bij, &bi); err != nil {
		return nil, err
	}
	return bi, nil
}

func getAllBlobInfos(tx *bolt.Tx) ([]*BlobInfo, error) {
	return getBlobInfosWithDigestPrefix(tx, "")
}

func getBlobInfosWithDigestPrefix(tx *bolt.Tx, digestPrefix string) ([]*BlobInfo, error) {
	b := tx.Bucket(blobinfobucket)
	if b == nil {
		return nil, fmt.Errorf("bucket does not exists")
	}

	var blobInfos []*BlobInfo

	c := b.Cursor()
	prefix := blobInfoDigestKey(digestPrefix)
	for k, v := c.Seek(prefix); bytes.HasPrefix(k, prefix); k, v = c.Next() {
		var bi *BlobInfo
		if err := json.Unmarshal(v, &bi); err != nil {
			return nil, err
		}
		blobInfos = append(blobInfos, bi)
	}

	return blobInfos, nil
}

func getBlobInfosByMediaType(tx *bolt.Tx, mediaType string) ([]*BlobInfo, error) {
	b := tx.Bucket(blobinfobucket)
	if b == nil {
		return nil, fmt.Errorf("bucket does not exists")
	}

	var blobInfos []*BlobInfo

	// Prefix scan by imageDigest
	prefix := blobInfoMediaTypeDigestKey(mediaType, "")
	c := b.Cursor()
	for k, _ := c.Seek(prefix); bytes.HasPrefix(k, prefix); k, _ = c.Next() {
		// Get the ID
		digest := path.Base(string(k))
		bi, err := getBlobInfo(tx, digest)
		if bi == nil {
			panic("not existent blob info entry")
		}
		if err != nil {
			return nil, err
		}
		blobInfos = append(blobInfos, bi)
	}
	return blobInfos, nil
}

func removeBlobInfo(tx *bolt.Tx, digest string) error {
	b := tx.Bucket(blobinfobucket)
	if b == nil {
		return fmt.Errorf("bucket does not exists")
	}

	bi, err := getBlobInfo(tx, digest)
	if err != nil {
		return err
	}
	if bi == nil {
		return nil
	}

	if err := b.Delete(blobInfoDigestKey(digest)); err != nil {
		return err
	}
	// Remove additional non unique index on mediaType for the specified id
	if err := b.Delete(blobInfoMediaTypeDigestKey(bi.MediaType, digest)); err != nil {
		return err
	}
	return nil
}
