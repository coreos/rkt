// Copyright 2017 The rkt Authors
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

package main

import (
	"testing"
	"time"

	"github.com/rkt/rkt/store/imagestore"
)

var (
	imageSize                 = 1073741824
	treeStoreSize             = 968884224
	gracePeriod               = 24 * time.Hour * 20
	impTime                   = time.Date(2017, time.January, 1, 1, 0, 0, 0, time.UTC)
	plusTenDays               = time.Date(2017, time.January, 10, 1, 0, 0, 0, time.UTC)
	plusTwentyDays            = time.Date(2017, time.January, 20, 1, 0, 0, 0, time.UTC)
	currentTime               = time.Date(2017, time.January, 9, 1, 0, 0, 0, time.UTC)
	imagesExpectedToBeRemoved = []string{
		"sha512-a000000000000000000000000000000000000000000000000000000000000003",
		"sha512-a000000000000000000000000000000000000000000000000000000000000006",
	}
)

func GetAllACIInfosTest() []*imagestore.ACIInfo {
	return []*imagestore.ACIInfo{
		{
			BlobKey:    "sha512-a000000000000000000000000000000000000000000000000000000000000001",
			Name:       "test.storage/image1",
			ImportTime: impTime,
			LastUsed:   plusTwentyDays,
			Latest:     true,
		},
		{
			BlobKey:    "sha512-a000000000000000000000000000000000000000000000000000000000000002",
			Name:       "test.storage/image2",
			ImportTime: impTime,
			LastUsed:   plusTenDays,
			Latest:     true,
		},
		{
			BlobKey:    "sha512-a000000000000000000000000000000000000000000000000000000000000003",
			Name:       "test.storage/image3",
			ImportTime: impTime,
			LastUsed:   plusTwentyDays,
			Latest:     false,
		},
		{
			BlobKey:    "sha512-a000000000000000000000000000000000000000000000000000000000000004",
			Name:       "test.storage/image3",
			ImportTime: impTime,
			LastUsed:   plusTenDays,
			Latest:     false,
		},
		{
			BlobKey:    "sha512-a000000000000000000000000000000000000000000000000000000000000005",
			Name:       "test.storage/image3",
			ImportTime: impTime,
			LastUsed:   plusTenDays,
			Latest:     true,
		},
		{
			BlobKey:    "sha512-a000000000000000000000000000000000000000000000000000000000000006",
			Name:       "test.storage/image3",
			ImportTime: impTime,
			LastUsed:   plusTwentyDays,
			Latest:     false,
		},
	}
}

func getRunningImagesTest() []string {
	runningImages := []string{
		"sha512-a000000000000000000000000000000000000000000000000000000000000001",
		"sha512-a000000000000000000000000000000000000000000000000000000000000002",
		"sha512-a000000000000000000000000000000000000000000000000000000000000005",
	}
	return runningImages
}

func TestGcStore(t *testing.T) {
	var imagesToRemove []string

	aciinfos := GetAllACIInfosTest()
	runningImages := getRunningImagesTest()

	for _, ai := range aciinfos {
		if currentTime.Sub(ai.LastUsed) <= gracePeriod {
			break
		}
		if main.isInSet(ai.BlobKey, runningImages) {
			continue
		}
		imagesToRemove = append(imagesToRemove, ai.BlobKey)
	}

	if &imagesToRemove != &imagesExpectedToBeRemoved {
		t.Errorf("some images are not being deleted properly!")
	}
}
