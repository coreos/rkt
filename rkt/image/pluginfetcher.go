// Copyright 2015 The rkt Authors
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
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"path"
	"time"

	"github.com/coreos/rkt/pkg/keystore"
	"github.com/coreos/rkt/rkt/config"
	rktflag "github.com/coreos/rkt/rkt/flag"
	"github.com/coreos/rkt/rkt/image/fetchers"
	"github.com/coreos/rkt/store"
)

const (
	pluginTemplate = "rkt-fetcher-%s"
)

type pluginFetcher struct {
	InsecureFlags *rktflag.SecFlags
	Auth          map[string]config.Headerer
	S             *store.Store
	Ks            *keystore.Keystore
	Debug         bool
	Rem           *store.Remote
}

func (f *pluginFetcher) Hash(u *url.URL) (string, error) {
	ensureLogger(f.Debug)

	pluginName := fmt.Sprintf(pluginTemplate, u.Scheme)

	_, err := exec.LookPath(pluginName)
	if err != nil {
		return "", fmt.Errorf("unable to find a plugin for the scheme %q", u.Scheme)
	}

	parentTmpDir, err := f.S.TmpDir()
	if err != nil {
		return "", err
	}
	tmpDir, err := ioutil.TempDir(parentTmpDir, "fetch-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	aciFilePath := path.Join(tmpDir, "aci")
	ascFilePath := path.Join(tmpDir, "asc")

	auth := make(map[string]fetchers.Headers)
	for host, headerer := range f.Auth {
		headers := make(fetchers.Headers)
		for k, v := range headerer.GetHeader() {
			headers[k] = v
		}
		auth[host] = headers
	}

	conf := &fetchers.Config{
		Version: 1,
		Scheme:  u.Scheme,
		Name:    path.Join(u.Host, u.Path),
		InsecureOpts: fetchers.InsecureOpts{
			AllowHTTP:      f.InsecureFlags.AllowHTTP(),
			SkipTLSCheck:   f.InsecureFlags.SkipTLSCheck(),
			SkipImageCheck: f.InsecureFlags.SkipImageCheck(),
		},
		Debug:         f.Debug,
		Headers:       auth,
		OutputACIPath: aciFilePath,
		OutputASCPath: ascFilePath,
	}
	confBlob, err := json.Marshal(conf)
	stdinBuffer := bytes.NewBuffer(confBlob)

	stdoutBuf := &bytes.Buffer{}

	cmd := exec.Command(pluginName)
	cmd.Stdin = stdinBuffer
	cmd.Stderr = os.Stderr
	cmd.Stdout = stdoutBuf
	err = cmd.Run()
	if err != nil {
		return "", err
	}

	resBlob := stdoutBuf.Bytes()
	res := &fetchers.Result{}
	err = json.Unmarshal(resBlob, res)
	if err != nil {
		return "", err
	}

	if f.Rem != nil && (res.UseCached || useCached(f.Rem.DownloadTime, res.MaxAge)) {
		return f.Rem.BlobKey, nil
	}

	aciFile, err := os.Open(aciFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("plugin didn't fetch an image")
		}
		return "", err
	}
	defer aciFile.Close()

	if !f.InsecureFlags.SkipImageCheck() {
		ascFile, err := os.Open(ascFilePath)
		if err != nil {
			if os.IsNotExist(err) {
				return "", fmt.Errorf("plugin didn't fetch a signature")
			}
			return "", err
		}
		defer ascFile.Close()

		v, err := newValidator(aciFile)
		if err != nil {
			return "", err
		}
		entity, err := v.ValidateWithSignature(f.Ks, ascFile)
		if err != nil {
			return "", err
		}
		if _, err := aciFile.Seek(0, 0); err != nil {
			return "", fmt.Errorf("error seeking ACI file: %v", err)
		}
		printIdentities(entity)
	}

	key, err := f.S.WriteACI(aciFile, res.Latest)
	if err != nil {
		return "", err
	}

	// TODO(krnowak): Consider dropping the signature URL part
	// from store.Remote. It is not used anywhere and the data
	// stored here is useless.
	newRem := store.NewRemote(u.String(), ascURLFromImgURL(u).String())
	newRem.BlobKey = key
	newRem.DownloadTime = time.Now()
	if res.ETag != "" {
		newRem.ETag = res.ETag
	}
	if res.MaxAge != 0 {
		newRem.CacheMaxAge = res.MaxAge
	}
	err = f.S.WriteRemote(newRem)
	if err != nil {
		return "", err
	}

	return key, nil
}
