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
	"net/url"
	"path"

	"github.com/boltdb/bolt"
)

const (
	refID     = "id"
	refDigest = "digest"
)

var (
	refbucket = []byte("ref")
)

// Ref contains the treestore information.
type Ref struct {
	// Ref name
	ID string
	// Blob id
	Digest string
}

func refIDKey(id string) []byte {
	return []byte(url.QueryEscape(refID) + "/" + id)
}

func refDigestIDKey(digest, id string) []byte {
	checkKeyPart(digest)
	if digest == "" {
		panic("empty digest value")
	}
	// don't use path.Join since we want to keep the / if id is empty
	return []byte(refDigest + "/" + digest + "/" + url.QueryEscape(id))
}

func writeRef(tx *bolt.Tx, ref *Ref) error {
	b := tx.Bucket(refbucket)
	if b == nil {
		return fmt.Errorf("bucket does not exists")
	}
	refj, err := json.Marshal(ref)
	if err != nil {
		return err
	}

	// Insert the json ref using ID as primary key
	b.Put(refIDKey(ref.ID), refj)

	// Add additional non unique index on digest, use "/" as
	// separator between Digest and ID
	b.Put(refDigestIDKey(ref.Digest, ref.ID), nil)

	return nil
}

func getRef(tx *bolt.Tx, id string) (*Ref, error) {
	b := tx.Bucket(refbucket)
	if b == nil {
		return nil, fmt.Errorf("bucket does not exists")
	}

	refj := b.Get(refIDKey(id))
	if refj == nil {
		return nil, nil
	}

	var ref *Ref
	if err := json.Unmarshal(refj, &ref); err != nil {
		return nil, err
	}
	return ref, nil
}

func getAllRefs(tx *bolt.Tx) ([]*Ref, error) {
	b := tx.Bucket(refbucket)
	if b == nil {
		return nil, fmt.Errorf("bucket does not exists")
	}

	var refs []*Ref

	c := b.Cursor()
	prefix := refIDKey("")
	for k, v := c.Seek(prefix); bytes.HasPrefix(k, prefix); k, v = c.Next() {
		var ref *Ref
		if err := json.Unmarshal(v, &ref); err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}

	return refs, nil
}

func getRefsByDigest(tx *bolt.Tx, digest string) ([]*Ref, error) {
	b := tx.Bucket(refbucket)
	if b == nil {
		return nil, fmt.Errorf("bucket does not exists")
	}

	var refs []*Ref

	// Prefix scan by imageDigest
	prefix := refDigestIDKey(digest, "")
	c := b.Cursor()
	for k, _ := c.Seek(prefix); bytes.HasPrefix(k, prefix); k, _ = c.Next() {
		// Get the ID
		id, err := url.QueryUnescape(path.Base(string(k)))
		if err != nil {
			return nil, err
		}
		ref, err := getRef(tx, id)
		if ref == nil {
			panic("not existent blob info entry")
		}
		if err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	return refs, nil
}

func removeRef(tx *bolt.Tx, id string) error {
	b := tx.Bucket(refbucket)
	if b == nil {
		return fmt.Errorf("bucket does not exists")
	}

	ref, err := getRef(tx, id)
	if err != nil {
		return err
	}
	if ref == nil {
		return nil
	}

	if err := b.Delete(refIDKey(id)); err != nil {
		return err
	}
	// Remove additional non unique index on Digest for the specified ID
	if err := b.Delete(refDigestIDKey(ref.Digest, id)); err != nil {
		return err
	}
	return nil
}
