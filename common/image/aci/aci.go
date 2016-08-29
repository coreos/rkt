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

package aci

import (
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/appc/spec/aci"
	"github.com/coreos/rkt/common/image"
	"github.com/coreos/rkt/common/mediatype"
	"github.com/coreos/rkt/pkg/digest"
	"github.com/coreos/rkt/store/casref/rwcasref"
	"github.com/hashicorp/errwrap"
)

// ACIInfo is used to store information about an ACI.
type ACIInfo struct {
}

// FullACIInfo merges BlobInfo with ACIInfo.
type FullACIInfo struct {
	rwcasref.BlobInfo
	image.ImageInfo
}

const (
	DataTypeACIInfo = "aciinfo"
)

// WriteACI takes an ACI encapsulated in an io.Reader, decompresses it if
// necessary, and then stores it in the store under a digest based on the hash
// of the uncompressed ACI.
func WriteACI(s *rwcasref.Store, r io.ReadSeeker) (string, error) {
	dr, err := aci.NewCompressedReader(r)
	if err != nil {
		return "", errwrap.Wrap(errors.New("error decompressing image"), err)
	}
	defer dr.Close()

	imageInfo := &image.ImageInfo{
		LastUsed: time.Now(),
	}
	iij, err := json.Marshal(imageInfo)
	if err != nil {
		return "", errwrap.Wrap(errors.New("error marshalling image info"), err)
	}
	blobData := map[string][]byte{
		image.DataTypeImageInfo: iij,
	}
	return s.WriteBlob(dr, string(mediatype.ACI), blobData, digest.SHA512)
}

func ReadACI(s *rwcasref.Store, digest string) (io.ReadCloser, error) {
	// TODO(sgotti) Check that the ACIInfo blob data exists. If not it
	// means something broke between writing the blob and setting the data.
	return s.ReadBlob(digest)
}

func GetFullACIInfo(s *rwcasref.Store, digest string) (*FullACIInfo, error) {
	bi, err := s.GetBlobInfo(digest)
	if err != nil {
		return nil, errwrap.Wrap(errors.New("error getting aci blob infos"), err)
	}
	if bi == nil {
		return nil, errors.New("blob doesn't exists")
	}
	return getFullACIInfo(s, bi)
}

func getFullACIInfo(s *rwcasref.Store, bi *rwcasref.BlobInfo) (*FullACIInfo, error) {
	aij, err := s.GetBlobData(bi.Digest, image.DataTypeImageInfo)
	if err != nil {
		return nil, errwrap.Wrap(errors.New("error getting aci blob infos"), err)
	}
	var ai image.ImageInfo
	if err := json.Unmarshal(aij, &ai); err != nil {
		return nil, errwrap.Wrap(errors.New("error unmarshalling aci info"), err)
	}
	fullACIInfo := &FullACIInfo{
		rwcasref.BlobInfo{
			Digest:     bi.Digest,
			MediaType:  bi.MediaType,
			ImportTime: bi.ImportTime,
			Size:       bi.Size,
		},
		image.ImageInfo{
			LastUsed: ai.LastUsed,
		},
	}
	return fullACIInfo, nil
}

func GetAllACIInfos(s *rwcasref.Store) ([]*FullACIInfo, error) {
	fullACIInfos := []*FullACIInfo{}
	blobInfos, err := s.GetBlobsInfosByMediaType(string(mediatype.ACI))
	if err != nil {
		return nil, errwrap.Wrap(errors.New("error getting aci blob infos"), err)
	}
	for _, bi := range blobInfos {
		fullACIInfo, err := getFullACIInfo(s, bi)
		if err != nil {
			return nil, errwrap.Wrap(errors.New("error getting aci info"), err)
		}
		fullACIInfos = append(fullACIInfos, fullACIInfo)
	}
	return fullACIInfos, nil
}
