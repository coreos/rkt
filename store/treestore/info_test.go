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
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/boltdb/bolt"
	"github.com/coreos/rkt/pkg/kvdb"
)

func TestWriteInfo(t *testing.T) {
	dir, err := ioutil.TempDir("", tstprefix)
	if err != nil {
		t.Fatalf("error creating tempdir: %v", err)
	}
	defer os.RemoveAll(dir)

	tests := []struct {
		infos   []*Info
		expkeys []string
	}{
		{
			[]*Info{
				{
					ID:          "id01",
					ImageDigest: "digest01",
				},
				{
					ID:          "id02",
					ImageDigest: "digest01",
				},
				{
					ID:          "id03",
					ImageDigest: "digest02",
				},
			},
			[]string{
				infoID + "/id01",
				infoID + "/id02",
				infoID + "/id03",
				infoImage + "/digest01/id01",
				infoImage + "/digest01/id02",
				infoImage + "/digest02/id03",
			},
		},
	}

	for _, tt := range tests {
		// Insert entries
		db := kvdb.NewDB(filepath.Join(dir, "db"), defaultFilePerm)
		// Create the "info" bucket
		if err := db.DoRW(func(tx *bolt.Tx) error {
			_, err := tx.CreateBucketIfNotExists([]byte(infobucket))
			return err
		}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if err := db.DoRW(func(tx *bolt.Tx) error {
			for _, info := range tt.infos {
				if err := writeInfo(tx, info); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
			return nil
		}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Check inserted entries are in the db
		var infos []*Info
		if err := db.DoRO(func(tx *bolt.Tx) error {
			var err error
			infos, err = getAllInfos(tx)
			if err != nil {
				return err
			}
			return nil
		}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(infos, tt.infos) {
			t.Errorf("expected infos %v, got %v", tt.infos, infos)
		}

		// Check expected keys are in the db
		keys := []string{}
		if err := db.DoRO(func(tx *bolt.Tx) error {
			b := tx.Bucket(infobucket)
			c := b.Cursor()
			for k, _ := c.First(); k != nil; k, _ = c.Next() {
				keys = append(keys, string(k))
			}
			return nil
		}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(keys, tt.expkeys) {
			t.Errorf("expected keys %s, got %s", tt.expkeys, keys)
		}
	}
}

func TestDeleteInfo(t *testing.T) {
	dir, err := ioutil.TempDir("", tstprefix)
	if err != nil {
		t.Fatalf("error creating tempdir: %v", err)
	}
	defer os.RemoveAll(dir)

	tests := []struct {
		infos     []*Info
		removeIds []string
		expkeys   []string
	}{
		{
			[]*Info{
				{
					ID:          "id01",
					ImageDigest: "digest01",
				},
				{
					ID:          "id02",
					ImageDigest: "digest01",
				},
				{
					ID:          "id03",
					ImageDigest: "digest02",
				},
			},
			[]string{"id01"},
			[]string{
				infoID + "/id02",
				infoID + "/id03",
				infoImage + "/digest01/id02",
				infoImage + "/digest02/id03",
			},
		},
		{
			[]*Info{
				{
					ID:          "id01",
					ImageDigest: "digest01",
				},
				{
					ID:          "id02",
					ImageDigest: "digest01",
				},
				{
					ID:          "id03",
					ImageDigest: "digest02",
				},
			},
			[]string{"nonexistentid"},
			[]string{
				infoID + "/id01",
				infoID + "/id02",
				infoID + "/id03",
				infoImage + "/digest01/id01",
				infoImage + "/digest01/id02",
				infoImage + "/digest02/id03",
			},
		},
	}

	for _, tt := range tests {
		// Insert entries
		db := kvdb.NewDB(filepath.Join(dir, "db"), defaultFilePerm)
		// Create the "info" bucket
		if err := db.DoRW(func(tx *bolt.Tx) error {
			_, err := tx.CreateBucketIfNotExists([]byte(infobucket))
			return err
		}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if err := db.DoRW(func(tx *bolt.Tx) error {
			for _, info := range tt.infos {
				if err := writeInfo(tx, info); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
			return nil
		}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Remove required ids
		if err := db.DoRW(func(tx *bolt.Tx) error {
			var err error
			for _, id := range tt.removeIds {
				if err = removeInfo(tx, id); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Check expected keys are in the db
		keys := []string{}
		if err := db.DoRO(func(tx *bolt.Tx) error {
			b := tx.Bucket(infobucket)
			c := b.Cursor()
			for k, _ := c.First(); k != nil; k, _ = c.Next() {
				keys = append(keys, string(k))
			}
			return nil
		}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(keys, tt.expkeys) {
			t.Errorf("expected keys %s, got %s", tt.expkeys, keys)
		}
	}
}

func TestGetImageInfoByImageDigest(t *testing.T) {
	dir, err := ioutil.TempDir("", tstprefix)
	if err != nil {
		t.Fatalf("error creating tempdir: %v", err)
	}
	defer os.RemoveAll(dir)

	tests := []struct {
		infos       []*Info
		imageDigest string
		expIds      []string
	}{
		{
			[]*Info{
				{
					ID:          "id01",
					ImageDigest: "digest01",
				},
				{
					ID:          "id02",
					ImageDigest: "digest01",
				},
				{
					ID:          "id03",
					ImageDigest: "digest02",
				},
			},
			"digest01",
			[]string{"id01", "id02"},
		},
		{
			[]*Info{
				{
					ID:          "id01",
					ImageDigest: "digest01",
				},
				{
					ID:          "id02",
					ImageDigest: "digest01",
				},
				{
					ID:          "id03",
					ImageDigest: "digest02",
				},
			},
			"digest03",
			[]string{},
		},
	}

	for _, tt := range tests {
		// Insert entries
		db := kvdb.NewDB(filepath.Join(dir, "db"), defaultFilePerm)
		// Create the "info" bucket
		if err := db.DoRW(func(tx *bolt.Tx) error {
			_, err := tx.CreateBucketIfNotExists([]byte(infobucket))
			return err
		}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if err := db.DoRW(func(tx *bolt.Tx) error {
			for _, info := range tt.infos {
				if err := writeInfo(tx, info); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
			return nil
		}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Get infos by image digest
		var infos []*Info
		if err := db.DoRW(func(tx *bolt.Tx) error {
			var err error
			infos, err = getInfosByImageDigest(tx, tt.imageDigest)
			return err
		}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		ids := []string{}
		for _, info := range infos {
			ids = append(ids, info.ID)
		}
		if !reflect.DeepEqual(ids, tt.expIds) {
			t.Errorf("expected ids %v, got %v", tt.expIds, ids)
		}
	}
}
