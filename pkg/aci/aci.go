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

// Package aci implements helper functions for working with ACIs
package aci

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/coreos/rocket/Godeps/_workspace/src/github.com/appc/spec/aci"
	"github.com/coreos/rocket/Godeps/_workspace/src/github.com/appc/spec/schema"
	"github.com/coreos/rocket/Godeps/_workspace/src/golang.org/x/crypto/openpgp"
)

type ACIEntry struct {
	Header   *tar.Header
	Contents string
}

// NewBasicACI creates a new ACI in the given directory with the given name.
// Used for testing.
func NewBasicACI(dir string, name string) (*os.File, error) {
	manifest := fmt.Sprintf(`{"acKind":"ImageManifest","acVersion":"0.1.1","name":"%s"}`, name)
	return NewACI(dir, manifest, nil)
}

// NewACI creates a new ACI in the given directory with the given image
// manifest and entries.
// Used for testing.
func NewACI(dir string, manifest string, entries []*ACIEntry) (*os.File, error) {
	var im schema.ImageManifest
	if err := im.UnmarshalJSON([]byte(manifest)); err != nil {
		return nil, err
	}

	tf, err := ioutil.TempFile(dir, "")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tf.Name())

	tw := tar.NewWriter(tf)
	aw := aci.NewImageWriter(im, tw)

	for _, entry := range entries {
		// Add default mode
		if entry.Header.Mode == 0 {
			if entry.Header.Typeflag == tar.TypeDir {
				entry.Header.Mode = 0755
			} else {
				entry.Header.Mode = 0644
			}
		}
		sr := strings.NewReader(entry.Contents)
		if err := aw.AddFile(entry.Header, sr); err != nil {
			return nil, err
		}
	}

	if err := aw.Close(); err != nil {
		return nil, err
	}
	return tf, nil
}

// NewDetachedSignature creates a new openpgp armored detached signature for the given ACI
// signed with armoredPrivateKey.
func NewDetachedSignature(armoredPrivateKey string, aci io.Reader) (io.Reader, error) {
	entityList, err := openpgp.ReadArmoredKeyRing(bytes.NewBufferString(armoredPrivateKey))
	if err != nil {
		return nil, err
	}
	if len(entityList) < 1 {
		return nil, errors.New("empty entity list")
	}
	signature := &bytes.Buffer{}
	if err := openpgp.ArmoredDetachSign(signature, entityList[0], aci, nil); err != nil {
		return nil, err
	}
	return signature, nil
}
