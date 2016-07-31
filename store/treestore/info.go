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

package treestore

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"github.com/boltdb/bolt"
)

const (
	infoID    = "id"
	infoImage = "image"
)

var (
	infobucket = []byte("info")
)

// Info contains the treestore information.
type Info struct {
	// Treestore id
	ID string
	// Digest of the rendered image. With ACI this is the hash of the top image
	ImageDigest string
	// Rendered image checksum
	Checksum string
	// Rendered image size
	Size int64
}

func checkInfoKey(k string) {
	if strings.Contains(k, "/") {
		panic("bad image value")
	}
}

func infoIDKey(id string) []byte {
	checkInfoKey(id)
	return []byte(infoID + "/" + id)
}

func infoImageKey(image, id string) []byte {
	checkInfoKey(image)
	checkInfoKey(id)
	if image == "" {
		panic("empty image value")
	}
	// don't use path.Join since we want to keep the / if id is empty
	return []byte(infoImage + "/" + image + "/" + id)
}

func writeInfo(tx *bolt.Tx, info *Info) error {
	b := tx.Bucket(infobucket)
	if b == nil {
		return fmt.Errorf("bucket does not exist")
	}
	ij, err := json.Marshal(info)
	if err != nil {
		return err
	}

	// Insert the json info using ID as primary key
	b.Put(infoIDKey(info.ID), ij)

	// Add additional non unique index on info.ImageDigest, use "/" as
	// separator between Image and ID
	b.Put(infoImageKey(info.ImageDigest, info.ID), nil)
	return nil
}

func getInfo(tx *bolt.Tx, id string) (*Info, error) {
	b := tx.Bucket(infobucket)
	if b == nil {
		return nil, fmt.Errorf("bucket does not exist")
	}

	ij := b.Get(infoIDKey(id))
	if ij == nil {
		return nil, nil
	}

	var i *Info
	if err := json.Unmarshal(ij, &i); err != nil {
		return nil, err
	}
	return i, nil
}

func getAllInfos(tx *bolt.Tx) ([]*Info, error) {
	b := tx.Bucket(infobucket)
	if b == nil {
		return nil, fmt.Errorf("bucket does not exist")
	}

	var infos []*Info

	c := b.Cursor()
	prefix := infoIDKey("")
	for k, v := c.Seek(prefix); bytes.HasPrefix(k, prefix); k, v = c.Next() {
		var i *Info
		if err := json.Unmarshal(v, &i); err != nil {
			return nil, err
		}
		infos = append(infos, i)
	}

	return infos, nil
}

func getInfosByImageDigest(tx *bolt.Tx, imageDigest string) ([]*Info, error) {
	b := tx.Bucket(infobucket)
	if b == nil {
		return nil, fmt.Errorf("bucket does not exist")
	}

	var infos []*Info

	// Prefix scan by imageDigest
	prefix := infoImageKey(imageDigest, "")
	c := b.Cursor()
	for k, _ := c.Seek(prefix); bytes.HasPrefix(k, prefix); k, _ = c.Next() {
		// Get the ID
		id := path.Base(string(k))
		i, err := getInfo(tx, id)
		if i == nil {
			panic("nonexistent tree store info entry")
		}
		if err != nil {
			return nil, err
		}
		infos = append(infos, i)
	}
	return infos, nil
}

func removeInfo(tx *bolt.Tx, id string) error {
	b := tx.Bucket(infobucket)
	if b == nil {
		return fmt.Errorf("bucket does not exist")
	}

	i, err := getInfo(tx, id)
	if err != nil {
		return err
	}
	if i == nil {
		return nil
	}

	if err := b.Delete(infoIDKey(id)); err != nil {
		return err
	}
	// Remove additional non unique index for the specified id
	if err := b.Delete(infoImageKey(i.ImageDigest, id)); err != nil {
		return err
	}

	return nil
}
