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
	"context"
	"fmt"
	"io"
	"os"

	docker2aci "github.com/appc/docker2aci/lib"
	d2acommon "github.com/appc/docker2aci/lib/common"
	dockerCli "github.com/docker/docker/client"
)

type dockerDaemonFetcher struct {
	*Fetcher
	ImageName string
}

func (d *dockerDaemonFetcher) Hash() (string, error) {
	// image verification is not supported for docker images so adding this check
	// incase the user has forgotten to give such a flag then indicate such
	if !d.InsecureFlags.SkipImageCheck() {
		return "", fmt.Errorf("signature verification for docker images is not supported (try --insecure-options=image)")
	}

	// fetch the image from docker's store as tar file
	tarFilePath, tarCleaner, err := d.dockerToTar()
	if err != nil {
		return "", err
	}
	defer tarCleaner()

	// convert the tar file into rkt readable ACI format
	aciFilePath, aciCleaner, err := d.tarToACI(tarFilePath)
	if err != nil {
		return "", err
	}
	defer aciCleaner()

	// now that we have the ACI format image import into rkt store
	return d.importToRktStore(aciFilePath)
}

// dockerToTar fetches the image from docker's store and save it as
// a tar file in temporary place
func (d *dockerDaemonFetcher) dockerToTar() (string, func(), error) {
	// create a docker client to interact with docker daemon
	cli, err := dockerCli.NewEnvClient()
	if err != nil {
		return "", nil, fmt.Errorf("creating the docker client: %v", err)
	}

	// fetch the image from docker's store
	tar, err := cli.ImageSave(
		context.Background(),
		[]string{d.ImageName},
	)
	if err != nil {
		return "", nil, fmt.Errorf("fetching the image from docker store: %v", err)
	}
	defer tar.Close()

	// create a temporary file to copy the tar data we just received
	tmpTarFile, err := d.S.TmpFile()
	if err != nil {
		return "", nil, fmt.Errorf("creating tar file: %v", err)
	}
	defer tmpTarFile.Close()

	// now copy that tar content into a temporary file
	if _, err = io.Copy(tmpTarFile, tar); err != nil {
		return "", nil, fmt.Errorf("copying to tar file: %v", err)
	}
	tmpTarFile.Close()

	path := tmpTarFile.Name()
	return path, func() {
		os.Remove(path)
	}, nil
}

// tarToACI converts the tar file fetched from docker's store into
// rkt readable ACI image format
func (d *dockerDaemonFetcher) tarToACI(tarFilePath string) (string, func(), error) {
	// we will save all the temporary artifacts in this directory
	tempDir, err := d.S.TmpDir()
	if err != nil {
		return "", nil, fmt.Errorf("creating temporary directory: %v", err)
	}

	// Now convert that tar file into aci
	out, err := docker2aci.ConvertSavedFile(tarFilePath, docker2aci.FileConfig{
		CommonConfig: docker2aci.CommonConfig{
			Squash:      true,
			OutputDir:   tempDir,
			TmpDir:      tempDir,
			Compression: d2acommon.GzipCompression,
		},
	})
	if err != nil {
		return "", nil, fmt.Errorf("converting tar to aci: %v", err)
	}

	return out[0], func() {
		os.RemoveAll(tempDir)
	}, nil
}

func (d *dockerDaemonFetcher) importToRktStore(aciFilePath string) (string, error) {
	// TODO: implement the way to handle the image security options
	return d.fetchSingleImageByPath(aciFilePath, nil)
}
