// Copyright 2018 The rkt Authors
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

package image

import (
	"fmt"
	rktflag "github.com/rkt/rkt/rkt/flag"
	"github.com/rkt/rkt/store/imagestore"
	"net/url"

	"errors"
	"github.com/appc/docker2aci/lib"
	d2acommon "github.com/appc/docker2aci/lib/common"
	"github.com/hashicorp/errwrap"
	"io/ioutil"
	"os"
	"path/filepath"
)

// dockerArchiveFetcher is used to fetch images from local tarballs. It uses
// a docker2aci library to perform this task.
type dockerArchiveFetcher struct {
	InsecureFlags *rktflag.SecFlags
	S             *imagestore.Store
	Debug         bool
}

// Hash uses docker2aci to convert docker archive to
// ACI, then stores it in the store and returns the hash.
func (f *dockerArchiveFetcher) Hash(archivePath string) (string, error) {
	ensureLogger(f.Debug)
	u, err := url.Parse(archivePath)
	if err != nil {
		return "", errwrap.Wrap(fmt.Errorf("failed to parse given path %q", archivePath), err)
	}

	p, err := filepath.Abs(u.Path)
	if err != nil {
		return "", errwrap.Wrap(fmt.Errorf("failed to get an absolute path for %q", u.Path), err)
	}

	return f.fetchImageFromArchive(p)
}

func (f *dockerArchiveFetcher) fetchImageFromArchive(path string) (string, error) {
	diag.Printf("converting docker archive at %s to ACI", path)
	aciFile, err := f.convertToACI(path)
	if err != nil {
		return "", err
	}

	defer aciFile.Close()

	key, err := f.S.WriteACI(aciFile, imagestore.ACIFetchInfo{
		Latest: false,
	})
	if err != nil {
		return "", err
	}

	return key, nil
}

func (f *dockerArchiveFetcher) convertToACI(path string) (*os.File, error) {
	tmpDir, err := f.getTmpDir()
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	config := docker2aci.FileConfig{
		CommonConfig: docker2aci.CommonConfig{
			Squash:      true,
			OutputDir:   tmpDir,
			TmpDir:      tmpDir,
			Compression: d2acommon.NoCompression,
		},
		DockerURL: "", // Default value for -image flag
		// TODO: Support for command line flags supported by docker2aci (?)
	}

	acis, err := docker2aci.ConvertSavedFile(path, config)
	if err != nil {
		return nil, errwrap.Wrap(errors.New("error converting docker image to ACI"), err)
	}

	aciFile, err := os.Open(acis[0])
	if err != nil {
		return nil, errwrap.Wrap(errors.New("error opening squashed ACI file"), err)
	}

	return aciFile, nil
}

func (f *dockerArchiveFetcher) getTmpDir() (string, error) {
	storeTmpDir, err := f.S.TmpDir()
	if err != nil {
		return "", errwrap.Wrap(errors.New("error creating temporary dir for docker to ACI conversion"), err)
	}
	return ioutil.TempDir(storeTmpDir, "docker2aci-")
}
