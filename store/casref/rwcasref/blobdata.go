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

	"github.com/boltdb/bolt"
)

const (
	blobDataDigest   = "digest"
	blobDataDataType = "datatype"
)

var (
	blobdatabucket = []byte("blobdata")
)

// BlobData contains the treestore information.
type BlobData struct {
	// Blob digest
	Digest string
	// DataType represent the type of blob contents
	DataType string
	// Data
	Data []byte
}

func blobDataDigestDataTypeKey(digest string, dataType string) []byte {
	checkKeyPart(digest)
	return []byte(blobDataDigest + "/" + digest)
}

func blobDataDataTypeDigestKey(dataType string, digest string) []byte {
	checkKeyPart(dataType)
	checkKeyPart(digest)
	if dataType == "" {
		panic("empty data type value")
	}
	// don't use path.Join since we want to keep the / if digest is empty
	return []byte(blobDataDataType + "/" + dataType + "/" + digest)
}

func writeBlobData(tx *bolt.Tx, bd *BlobData) error {
	b := tx.Bucket(blobdatabucket)
	if b == nil {
		return fmt.Errorf("bucket does not exists")
	}
	bdj, err := json.Marshal(bd)
	if err != nil {
		return err
	}

	// Insert the json blobdata using Digest as primary key
	b.Put(blobDataDigestDataTypeKey(bd.Digest, bd.DataType), bdj)

	// Add additional non unique index on dataType, use "/" as
	// separator between DataType and Digest
	b.Put(blobDataDataTypeDigestKey(bd.DataType, bd.Digest), nil)

	return nil
}

func getBlobData(tx *bolt.Tx, digest string, dataType string) (*BlobData, error) {
	b := tx.Bucket(blobdatabucket)
	if b == nil {
		return nil, fmt.Errorf("bucket does not exists")
	}

	bdj := b.Get(blobDataDigestDataTypeKey(digest, dataType))
	if bdj == nil {
		return nil, nil
	}

	var bd *BlobData
	if err := json.Unmarshal(bdj, &bd); err != nil {
		return nil, err
	}
	return bd, nil
}

func getAllBlobDatasByDataType(tx *bolt.Tx, dataType string) ([]*BlobData, error) {
	return getBlobDatasWithDigestPrefix(tx, "", dataType)
}

func getBlobDatasWithDigestPrefix(tx *bolt.Tx, digestPrefix string, dataType string) ([]*BlobData, error) {
	b := tx.Bucket(blobdatabucket)
	if b == nil {
		return nil, fmt.Errorf("bucket does not exists")
	}

	var blobDatas []*BlobData

	c := b.Cursor()
	prefix := blobDataDigestDataTypeKey(digestPrefix, dataType)
	for k, v := c.Seek(prefix); bytes.HasPrefix(k, prefix); k, v = c.Next() {
		var bd *BlobData
		if err := json.Unmarshal(v, &bd); err != nil {
			return nil, err
		}
		blobDatas = append(blobDatas, bd)
	}

	return blobDatas, nil
}

func getBlobDatasByDataType(tx *bolt.Tx, dataType string) ([]*BlobData, error) {
	b := tx.Bucket(blobdatabucket)
	if b == nil {
		return nil, fmt.Errorf("bucket does not exists")
	}

	var blobDatas []*BlobData

	// Prefix scan by Digest
	prefix := blobDataDataTypeDigestKey(dataType, "")
	c := b.Cursor()
	for k, _ := c.Seek(prefix); bytes.HasPrefix(k, prefix); k, _ = c.Next() {
		// Get the ID
		digest := path.Base(string(k))
		bd, err := getBlobData(tx, digest, dataType)
		if bd == nil {
			panic("not existent blob info entry")
		}
		if err != nil {
			return nil, err
		}
		blobDatas = append(blobDatas, bd)
	}
	return blobDatas, nil
}

func removeBlobData(tx *bolt.Tx, digest string, dataType string) error {
	b := tx.Bucket(blobdatabucket)
	if b == nil {
		return fmt.Errorf("bucket does not exists")
	}

	bd, err := getBlobData(tx, digest, dataType)
	if err != nil {
		return err
	}
	if bd == nil {
		return nil
	}

	if err := b.Delete(blobDataDigestDataTypeKey(digest, dataType)); err != nil {
		return err
	}
	// Remove additional non unique index on dataType for the specified digest
	if err := b.Delete(blobDataDataTypeDigestKey(bd.DataType, digest)); err != nil {
		return err
	}
	return nil
}
