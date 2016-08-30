// Copyright 2015 The appc Authors
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

package docker2aci

import (
	"crypto/sha512"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"os"

	"github.com/appc/spec/aci"
	"github.com/appc/spec/schema"
	"github.com/appc/spec/schema/types"
)

const (
	hashPrefix = "sha512-"
)

type aciInfo struct {
	path          string
	key           string
	ImageManifest *schema.ImageManifest
}

// conversionStore is an simple implementation of the acirenderer.ACIRegistry
// interface. It stores the Docker layers converted to ACI so we can take
// advantage of acirenderer to generate a squashed ACI Image.
type conversionStore struct {
	acis map[string]*aciInfo
}

func newConversionStore() *conversionStore {
	return &conversionStore{acis: make(map[string]*aciInfo)}
}

func (ms *conversionStore) WriteACI(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	cr, err := aci.NewCompressedReader(f)
	if err != nil {
		return "", err
	}
	defer cr.Close()

	h := sha512.New()
	r := io.TeeReader(cr, h)

	// read the file so we can get the hash
	if _, err := io.Copy(ioutil.Discard, r); err != nil {
		return "", fmt.Errorf("error reading ACI: %v", err)
	}

	im, err := aci.ManifestFromImage(f)
	if err != nil {
		return "", err
	}

	key := ms.HashToKey(h)
	ms.acis[key] = &aciInfo{path: path, key: key, ImageManifest: im}
	return key, nil
}

func (ms *conversionStore) GetImageManifest(key string) (*schema.ImageManifest, error) {
	aci, ok := ms.acis[key]
	if !ok {
		return nil, fmt.Errorf("aci with key: %s not found", key)
	}
	return aci.ImageManifest, nil
}

func (ms *conversionStore) GetACI(name types.ACIdentifier, labels types.Labels) (string, error) {
	for _, aci := range ms.acis {
		// we implement this function to comply with the interface so don't
		// bother implementing a proper label check
		if aci.ImageManifest.Name.String() == name.String() {
			return aci.key, nil
		}
	}
	return "", fmt.Errorf("aci not found")
}

func (ms *conversionStore) ReadStream(key string) (io.ReadCloser, error) {
	img, ok := ms.acis[key]
	if !ok {
		return nil, fmt.Errorf("stream for key: %s not found", key)
	}
	f, err := os.Open(img.path)
	if err != nil {
		return nil, fmt.Errorf("error opening aci: %s", img.path)
	}

	tr, err := aci.NewCompressedReader(f)
	if err != nil {
		return nil, err
	}

	return tr, nil
}

func (ms *conversionStore) ResolveKey(key string) (string, error) {
	return key, nil
}

func (ms *conversionStore) HashToKey(h hash.Hash) string {
	s := h.Sum(nil)
	return fmt.Sprintf("%s%x", hashPrefix, s)
}
