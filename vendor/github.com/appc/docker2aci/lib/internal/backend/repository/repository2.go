// Copyright 2016 The appc Authors
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

package repository

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/appc/docker2aci/lib/common"
	"github.com/appc/docker2aci/lib/internal"
	"github.com/appc/docker2aci/lib/internal/types"
	"github.com/appc/docker2aci/lib/internal/typesV2"
	"github.com/appc/docker2aci/lib/internal/util"
	"github.com/appc/docker2aci/pkg/log"
	"github.com/appc/spec/schema"
	"github.com/coreos/pkg/progressutil"
)

const (
	defaultIndexURL = "registry-1.docker.io"
)

var validHex = regexp.MustCompile(`^([a-f0-9]{64})$`)

// A manifest conforming to the docker v2.1 spec
type v2Manifest struct {
	Name     string `json:"name"`
	Tag      string `json:"tag"`
	FSLayers []struct {
		BlobSum string `json:"blobSum"`
	} `json:"fsLayers"`
	History []struct {
		V1Compatibility string `json:"v1Compatibility"`
	} `json:"history"`
	Signature []byte `json:"signature"`
}

func (rb *RepositoryBackend) getImageInfoV2(dockerURL *types.ParsedDockerURL) ([]string, *types.ParsedDockerURL, error) {
	layers, err := rb.getManifestV2(dockerURL)
	if err != nil {
		return nil, nil, err
	}

	return layers, dockerURL, nil
}

func (rb *RepositoryBackend) buildACIV2(layerIDs []string, dockerURL *types.ParsedDockerURL, outputDir string, tmpBaseDir string, compression common.Compression) ([]string, []*schema.ImageManifest, error) {
	_, isVersion22 := rb.imageV2Manifests[*dockerURL]
	if isVersion22 {
		return rb.buildACIV22(layerIDs, dockerURL, outputDir, tmpBaseDir, compression)
	}
	return rb.buildACIV21(layerIDs, dockerURL, outputDir, tmpBaseDir, compression)
}

func (rb *RepositoryBackend) buildACIV21(layerIDs []string, dockerURL *types.ParsedDockerURL, outputDir string, tmpBaseDir string, compression common.Compression) ([]string, []*schema.ImageManifest, error) {
	layerFiles := make([]*os.File, len(layerIDs))
	layerDatas := make([]types.DockerImageData, len(layerIDs))

	tmpParentDir, err := ioutil.TempDir(tmpBaseDir, "docker2aci-")
	if err != nil {
		return nil, nil, err
	}
	defer os.RemoveAll(tmpParentDir)

	copier := progressutil.NewCopyProgressPrinter()

	var errChannels []chan error
	closers := make([]io.ReadCloser, len(layerIDs))
	var wg sync.WaitGroup
	for i, layerID := range layerIDs {
		wg.Add(1)
		errChan := make(chan error, 1)
		errChannels = append(errChannels, errChan)
		// https://github.com/golang/go/wiki/CommonMistakes
		i := i // golang--
		layerID := layerID
		go func() {
			defer wg.Done()

			manifest := rb.imageManifests[*dockerURL]

			layerIndex, ok := rb.layersIndex[layerID]
			if !ok {
				errChan <- fmt.Errorf("layer not found in manifest: %s", layerID)
				return
			}

			if len(manifest.History) <= layerIndex {
				errChan <- fmt.Errorf("history not found for layer %s", layerID)
				return
			}

			layerDatas[i] = types.DockerImageData{}
			if err := json.Unmarshal([]byte(manifest.History[layerIndex].V1Compatibility), &layerDatas[i]); err != nil {
				errChan <- fmt.Errorf("error unmarshaling layer data: %v", err)
				return
			}

			tmpDir, err := ioutil.TempDir(tmpParentDir, "")
			if err != nil {
				errChan <- fmt.Errorf("error creating dir: %v", err)
				return
			}

			layerFiles[i], closers[i], err = rb.getLayerV2(layerID, dockerURL, tmpDir, copier)
			if err != nil {
				errChan <- fmt.Errorf("error getting the remote layer: %v", err)
				return
			}
			errChan <- nil
		}()
	}
	// Need to wait for all of the readers to be added to the copier (which happens during rb.getLayerV2)
	wg.Wait()
	err = copier.PrintAndWait(os.Stderr, 500*time.Millisecond, nil)
	if err != nil {
		return nil, nil, err
	}
	for _, closer := range closers {
		if closer != nil {
			closer.Close()
		}
	}
	for _, errChan := range errChannels {
		err := <-errChan
		if err != nil {
			return nil, nil, err
		}
	}
	for _, layerFile := range layerFiles {
		err := layerFile.Sync()
		if err != nil {
			return nil, nil, err
		}
	}
	var aciLayerPaths []string
	var aciManifests []*schema.ImageManifest
	var curPwl []string
	for i := len(layerIDs) - 1; i >= 0; i-- {
		log.Debug("Generating layer ACI...")
		aciPath, aciManifest, err := internal.GenerateACI(i, layerDatas[i], dockerURL, outputDir, layerFiles[i], curPwl, compression)
		if err != nil {
			return nil, nil, fmt.Errorf("error generating ACI: %v", err)
		}
		aciLayerPaths = append(aciLayerPaths, aciPath)
		aciManifests = append(aciManifests, aciManifest)
		curPwl = aciManifest.PathWhitelist

		layerFiles[i].Close()
	}

	return aciLayerPaths, aciManifests, nil
}

type layer struct {
	index  int
	file   *os.File
	closer io.Closer
	err    error
}

func (rb *RepositoryBackend) buildACIV22(layerIDs []string, dockerURL *types.ParsedDockerURL, outputDir string, tmpBaseDir string, compression common.Compression) ([]string, []*schema.ImageManifest, error) {
	layerFiles := make([]*os.File, len(layerIDs))

	tmpParentDir, err := ioutil.TempDir(tmpBaseDir, "docker2aci-")
	if err != nil {
		return nil, nil, err
	}
	defer os.RemoveAll(tmpParentDir)

	copier := progressutil.NewCopyProgressPrinter()

	resultChan := make(chan layer, len(layerIDs))
	for i, layerID := range layerIDs {
		// https://github.com/golang/go/wiki/CommonMistakes
		i := i // golang--
		layerID := layerID
		go func() {
			tmpDir, err := ioutil.TempDir(tmpParentDir, "")
			if err != nil {
				resultChan <- layer{
					index: i,
					err:   fmt.Errorf("error creating dir: %v", err),
				}
				return
			}

			layerFile, closer, err := rb.getLayerV2(layerID, dockerURL, tmpDir, copier)
			if err != nil {
				resultChan <- layer{
					index: i,
					err:   fmt.Errorf("error getting the remote layer: %v", err),
				}
				return
			}
			resultChan <- layer{
				index:  i,
				file:   layerFile,
				closer: closer,
				err:    nil,
			}
		}()
	}
	var errs []error
	for i := 0; i < len(layerIDs); i++ {
		res := <-resultChan
		if res.closer != nil {
			defer res.closer.Close()
		}
		if res.file != nil {
			defer res.file.Close()
		}
		if res.err != nil {
			errs = append(errs, res.err)
		}
		layerFiles[res.index] = res.file
	}
	if len(errs) > 0 {
		return nil, nil, errs[0]
	}
	err = copier.PrintAndWait(os.Stderr, 500*time.Millisecond, nil)
	if err != nil {
		return nil, nil, err
	}
	for _, layerFile := range layerFiles {
		err := layerFile.Sync()
		if err != nil {
			return nil, nil, err
		}
	}
	var aciLayerPaths []string
	var aciManifests []*schema.ImageManifest
	var curPwl []string
	var i int
	for i = 0; i < len(layerIDs)-1; i++ {
		log.Debug("Generating layer ACI...")
		aciPath, aciManifest, err := internal.GenerateACI22LowerLayer(dockerURL, layerIDs[i], outputDir, layerFiles[i], curPwl, compression)
		if err != nil {
			return nil, nil, fmt.Errorf("error generating ACI: %v", err)
		}
		aciLayerPaths = append(aciLayerPaths, aciPath)
		aciManifests = append(aciManifests, aciManifest)
		curPwl = aciManifest.PathWhitelist
	}
	log.Debug("Generating layer ACI...")
	aciPath, aciManifest, err := internal.GenerateACI22TopLayer(dockerURL, rb.imageConfigs[*dockerURL], layerIDs[i], outputDir, layerFiles[i], curPwl, compression, aciManifests)
	if err != nil {
		return nil, nil, fmt.Errorf("error generating ACI: %v", err)
	}
	aciLayerPaths = append(aciLayerPaths, aciPath)
	aciManifests = append(aciManifests, aciManifest)

	return aciLayerPaths, aciManifests, nil
}

func (rb *RepositoryBackend) getManifestV2(dockerURL *types.ParsedDockerURL) ([]string, error) {
	var reference string
	if dockerURL.Digest != "" {
		reference = dockerURL.Digest
	} else {
		reference = dockerURL.Tag
	}
	url := rb.schema + path.Join(dockerURL.IndexURL, "v2", dockerURL.ImageName, "manifests", reference)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	rb.setBasicAuth(req)

	accepting := []string{
		typesV2.MediaTypeOCIManifest,
		typesV2.MediaTypeDockerV22Manifest,
		typesV2.MediaTypeDockerV21Manifest,
	}

	res, err := rb.makeRequest(req, dockerURL.ImageName, accepting)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected http code: %d, URL: %s", res.StatusCode, req.URL)
	}

	switch res.Header.Get("content-type") {
	case typesV2.MediaTypeDockerV22Manifest, typesV2.MediaTypeOCIManifest:
		return rb.getManifestV22(dockerURL, res)
	case typesV2.MediaTypeDockerV21Manifest:
		return rb.getManifestV21(dockerURL, res)
	}
	return rb.getManifestV21(dockerURL, res)
}

func (rb *RepositoryBackend) getManifestV21(dockerURL *types.ParsedDockerURL, res *http.Response) ([]string, error) {
	manblob, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	manifest := &v2Manifest{}

	err = json.Unmarshal(manblob, manifest)
	if err != nil {
		return nil, err
	}

	if manifest.Name != dockerURL.ImageName {
		return nil, fmt.Errorf("name doesn't match what was requested, expected: %s, downloaded: %s", dockerURL.ImageName, manifest.Name)
	}

	if manifest.Tag != dockerURL.Tag {
		return nil, fmt.Errorf("tag doesn't match what was requested, expected: %s, downloaded: %s", dockerURL.Tag, manifest.Tag)
	}

	if err := fixManifestLayers(manifest); err != nil {
		return nil, err
	}

	//TODO: verify signature here

	layers := make([]string, len(manifest.FSLayers))

	for i, layer := range manifest.FSLayers {
		if _, ok := rb.layersIndex[layer.BlobSum]; !ok {
			rb.layersIndex[layer.BlobSum] = i
		}
		layers[i] = layer.BlobSum
	}

	rb.imageManifests[*dockerURL] = *manifest

	return layers, nil
}

func (rb *RepositoryBackend) getManifestV22(dockerURL *types.ParsedDockerURL, res *http.Response) ([]string, error) {
	manblob, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	manifest := &typesV2.ImageManifest{}

	err = json.Unmarshal(manblob, manifest)
	if err != nil {
		return nil, err
	}

	//TODO: verify signature here

	layers := make([]string, len(manifest.Layers))

	for i, layer := range manifest.Layers {
		layers[i] = layer.Digest
	}

	err = rb.getConfigV22(dockerURL, manifest.Config.Digest)
	if err != nil {
		return nil, err
	}

	rb.imageV2Manifests[*dockerURL] = manifest

	return layers, nil
}

func (rb *RepositoryBackend) getConfigV22(dockerURL *types.ParsedDockerURL, configDigest string) error {
	url := rb.schema + path.Join(dockerURL.IndexURL, "v2", dockerURL.ImageName, "blobs", configDigest)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	rb.setBasicAuth(req)

	accepting := []string{
		typesV2.MediaTypeOCIConfig,
		typesV2.MediaTypeDockerV22Config,
	}

	res, err := rb.makeRequest(req, dockerURL.ImageName, accepting)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	confblob, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}
	config := &typesV2.ImageConfig{}
	err = json.Unmarshal(confblob, config)
	if err != nil {
		return err
	}
	rb.imageConfigs[*dockerURL] = config
	return nil
}

func fixManifestLayers(manifest *v2Manifest) error {
	type imageV1 struct {
		ID     string
		Parent string
	}
	imgs := make([]*imageV1, len(manifest.FSLayers))
	for i := range manifest.FSLayers {
		img := &imageV1{}

		if err := json.Unmarshal([]byte(manifest.History[i].V1Compatibility), img); err != nil {
			return err
		}

		imgs[i] = img
		if err := validateV1ID(img.ID); err != nil {
			return err
		}
	}

	if imgs[len(imgs)-1].Parent != "" {
		return errors.New("Invalid parent ID in the base layer of the image.")
	}

	// check general duplicates to error instead of a deadlock
	idmap := make(map[string]struct{})

	var lastID string
	for _, img := range imgs {
		// skip IDs that appear after each other, we handle those later
		if _, exists := idmap[img.ID]; img.ID != lastID && exists {
			return fmt.Errorf("ID %+v appears multiple times in manifest", img.ID)
		}
		lastID = img.ID
		idmap[lastID] = struct{}{}
	}

	// backwards loop so that we keep the remaining indexes after removing items
	for i := len(imgs) - 2; i >= 0; i-- {
		if imgs[i].ID == imgs[i+1].ID { // repeated ID. remove and continue
			manifest.FSLayers = append(manifest.FSLayers[:i], manifest.FSLayers[i+1:]...)
			manifest.History = append(manifest.History[:i], manifest.History[i+1:]...)
		} else if imgs[i].Parent != imgs[i+1].ID {
			return fmt.Errorf("Invalid parent ID. Expected %v, got %v.", imgs[i+1].ID, imgs[i].Parent)
		}
	}

	return nil
}

func validateV1ID(id string) error {
	if ok := validHex.MatchString(id); !ok {
		return fmt.Errorf("image ID %q is invalid", id)
	}
	return nil
}

func (rb *RepositoryBackend) getLayerV2(layerID string, dockerURL *types.ParsedDockerURL, tmpDir string, copier *progressutil.CopyProgressPrinter) (*os.File, io.ReadCloser, error) {
	var (
		err error
		res *http.Response
		url = rb.schema + path.Join(dockerURL.IndexURL, "v2", dockerURL.ImageName, "blobs", layerID)
	)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, nil, err
	}

	rb.setBasicAuth(req)

	accepting := []string{
		typesV2.MediaTypeDockerV22RootFS,
		typesV2.MediaTypeOCIRootFS,
	}

	res, err = rb.makeRequest(req, dockerURL.ImageName, accepting)
	if err != nil {
		return nil, nil, err
	}

	defer func() {
		if err != nil && res != nil {
			res.Body.Close()
		}
	}()

	if res.StatusCode == http.StatusTemporaryRedirect || res.StatusCode == http.StatusFound {
		location := res.Header.Get("Location")
		if location != "" {
			req, err = http.NewRequest("GET", location, nil)
			if err != nil {
				return nil, nil, err
			}
			res.Body.Close()
			res = nil
			res, err = rb.makeRequest(req, dockerURL.ImageName, accepting)
			if err != nil {
				return nil, nil, err
			}
		}
	}

	if res.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("HTTP code: %d. URL: %s", res.StatusCode, req.URL)
	}

	var in io.Reader
	in = res.Body

	var size int64

	if hdr := res.Header.Get("Content-Length"); hdr != "" {
		size, err = strconv.ParseInt(hdr, 10, 64)
		if err != nil {
			return nil, nil, err
		}
	}

	name := "Downloading " + layerID[:18]

	layerFile, err := ioutil.TempFile(tmpDir, "dockerlayer-")
	if err != nil {
		return nil, nil, err
	}

	err = copier.AddCopy(in, name, size, layerFile)
	if err != nil {
		return nil, nil, err
	}

	return layerFile, res.Body, nil
}

func (rb *RepositoryBackend) makeRequest(req *http.Request, repo string, acceptHeaders []string) (*http.Response, error) {
	setBearerHeader := false
	hostAuthTokens, ok := rb.hostsV2AuthTokens[req.URL.Host]
	if ok {
		authToken, ok := hostAuthTokens[repo]
		if ok {
			req.Header.Set("Authorization", "Bearer "+authToken)
			setBearerHeader = true
		}
	}

	for _, acceptHeader := range acceptHeaders {
		req.Header.Add("Accept", acceptHeader)
	}

	client := util.GetTLSClient(rb.insecure.SkipVerify)
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if res.StatusCode == http.StatusUnauthorized && setBearerHeader {
		return res, err
	}

	hdr := res.Header.Get("www-authenticate")
	if res.StatusCode != http.StatusUnauthorized || hdr == "" {
		return res, err
	}

	tokens := strings.Split(hdr, ",")
	if len(tokens) != 3 ||
		!strings.HasPrefix(strings.ToLower(tokens[0]), "bearer realm") {
		return res, err
	}
	res.Body.Close()

	var realm, service, scope string
	for _, token := range tokens {
		if strings.HasPrefix(strings.ToLower(token), "bearer realm") {
			realm = strings.Trim(token[len("bearer realm="):], "\"")
		}
		if strings.HasPrefix(token, "service") {
			service = strings.Trim(token[len("service="):], "\"")
		}
		if strings.HasPrefix(token, "scope") {
			scope = strings.Trim(token[len("scope="):], "\"")
		}
	}

	if realm == "" {
		return nil, fmt.Errorf("missing realm in bearer auth challenge")
	}
	if service == "" {
		return nil, fmt.Errorf("missing service in bearer auth challenge")
	}
	// The scope can be empty if we're not getting a token for a specific repo
	if scope == "" && repo != "" {
		// If the scope is empty and it shouldn't be, we can infer it based on the repo
		scope = fmt.Sprintf("repository:%s:pull", repo)
	}

	authReq, err := http.NewRequest("GET", realm, nil)
	if err != nil {
		return nil, err
	}

	getParams := authReq.URL.Query()
	getParams.Add("service", service)
	if scope != "" {
		getParams.Add("scope", scope)
	}
	authReq.URL.RawQuery = getParams.Encode()

	rb.setBasicAuth(authReq)

	res, err = client.Do(authReq)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	switch res.StatusCode {
	case http.StatusUnauthorized:
		return nil, fmt.Errorf("unable to retrieve auth token: 401 unauthorized")
	case http.StatusOK:
		break
	default:
		return nil, fmt.Errorf("unexpected http code: %d, URL: %s", res.StatusCode, authReq.URL)
	}

	tokenBlob, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	tokenStruct := struct {
		Token string `json:"token"`
	}{}

	err = json.Unmarshal(tokenBlob, &tokenStruct)
	if err != nil {
		return nil, err
	}

	hostAuthTokens, ok = rb.hostsV2AuthTokens[req.URL.Host]
	if !ok {
		hostAuthTokens = make(map[string]string)
		rb.hostsV2AuthTokens[req.URL.Host] = hostAuthTokens
	}

	hostAuthTokens[repo] = tokenStruct.Token

	return rb.makeRequest(req, repo, acceptHeaders)
}

func (rb *RepositoryBackend) setBasicAuth(req *http.Request) {
	if rb.username != "" && rb.password != "" {
		req.SetBasicAuth(rb.username, rb.password)
	}
}
