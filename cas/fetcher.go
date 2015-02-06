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

// Package cas implements a content-addressable-store on disk.
// It leverages the `diskv` package to store items in a simple
// key-value blob store: https://github.com/peterbourgon/diskv
package cas

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	neturl "net/url"
	"os"
	"time"

	"github.com/coreos/rocket/pkg/keystore"

	"github.com/coreos/rocket/Godeps/_workspace/src/github.com/appc/spec/aci"
	"github.com/coreos/rocket/Godeps/_workspace/src/github.com/appc/spec/schema/types"
	"github.com/coreos/rocket/Godeps/_workspace/src/github.com/mitchellh/ioprogress"
	"github.com/coreos/rocket/Godeps/_workspace/src/golang.org/x/crypto/openpgp"
)

func NewFetcher(aciurl, sigurl string) *Fetcher {
	r := &Fetcher{
		ACIURL: aciurl,
		SigURL: sigurl,
	}
	return r
}

type Fetcher struct {
	ACIURL string
	SigURL string
	ETag   string
	// The key in the blob store under which the ACI has been saved.
	BlobKey string
}

func (r Fetcher) Marshal() []byte {
	m, _ := json.Marshal(r)
	return m
}

func (r *Fetcher) Unmarshal(data []byte) {
	err := json.Unmarshal(data, r)
	if err != nil {
		panic(err)
	}
}

func (r Fetcher) Hash() string {
	return types.NewHashSHA512([]byte(r.ACIURL)).String()
}

func (r Fetcher) Type() int64 {
	return remoteType
}

// Get retrieves and verifies the remote ACI.
// If Keystore is nil signature verification will be skipped.
// Get returns the signer, an *os.File representing the ACI, a boolean indicating if the ACI was remote, and an error if any.
// err will be nil if the ACI gets successfully and the ACI is verified.
func (r Fetcher) Get(ds Store, ks *keystore.Keystore) (*openpgp.Entity, *os.File, bool, error) {
	var entity *openpgp.Entity
	var err error

	acif, isRemoteACI, err := getACI(ds, r.ACIURL)
	if err != nil {
		return nil, acif, isRemoteACI, fmt.Errorf("error getting the aci image: %v", err)
	}

	if ks != nil {
		fmt.Printf("Getting signature from %v\n", r.SigURL)
		sigFile, isRemoteSig, err := getSignatureFile(r.SigURL)
		if err != nil {
			return nil, acif, isRemoteACI, fmt.Errorf("error getting the signature file: %v, disable verification to ignore.", err)
		}
		defer sigFile.Close()
		if isRemoteSig { // on remote urls the returned file should be cleaned up
			defer os.Remove(sigFile.Name())
		}

		manifest, err := aci.ManifestFromImage(acif)
		if err != nil {
			return nil, acif, isRemoteACI, err
		}

		if _, err := acif.Seek(0, 0); err != nil {
			return nil, acif, isRemoteACI, err
		}
		if _, err := sigFile.Seek(0, 0); err != nil {
			return nil, acif, isRemoteACI, err
		}
		if entity, err = ks.CheckSignature(manifest.Name.String(), acif, sigFile); err != nil {
			return nil, acif, isRemoteACI, err
		}
	}

	if _, err := acif.Seek(0, 0); err != nil {
		return nil, acif, isRemoteACI, err
	}

	return entity, acif, isRemoteACI, nil
}

// TODO: add locking
// Store stores the ACI represented by r in the target data store.
func (r Fetcher) Store(ds Store, aci io.Reader) (*Fetcher, error) {
	key, err := ds.WriteACI(aci)
	if err != nil {
		return nil, err
	}
	r.BlobKey = key
	ds.WriteIndex(&r)
	return &r, nil
}

// getACI gets the aci specified at aciurl
func getACI(ds Store, aciurl string) (*os.File, bool, error) {
	return getURL(aciurl, "aci", ds.tmpFile)
}

// getSignatureFile gets the signature specified at sigurl
func getSignatureFile(sigurl string) (*os.File, bool, error) {
	getTemp := func() (*os.File, error) {
		return ioutil.TempFile("", "")
	}

	return getURL(sigurl, "sig", getTemp)
}

// getURL retrieves url, creating a temp file if necessary
// file:// http:// and https:// urls supported
func getURL(url, label string, getTempFile func() (*os.File, error)) (*os.File, bool, error) {
	// local urls are simply opened in-place
	u, err := neturl.Parse(url)
	if err != nil {
		return nil, false, fmt.Errorf("error parsing %s url: %v", label, err)
	}
	if u.Scheme == "file" {
		f, err := os.Open(u.Path)
		if err != nil {
			err = fmt.Errorf("error opening %s file: %v", label, err)
		}
		return f, false, err
	}

	// for remote urls create a temp file and cache the remote there
	tmp, err := getTempFile()
	if err != nil {
		return nil, true, fmt.Errorf("error downloading %s: %v", label, err)
	}

	if err = downloadHTTP(url, label, tmp); err != nil {
		os.Remove(tmp.Name())
		tmp.Close()
		return nil, true, fmt.Errorf("error downloading %s: %v", label, err)
	}

	return tmp, true, nil
}

// downloadHTTP retrieves url via http storing it at output
func downloadHTTP(url, label string, output *os.File) error {
	res, err := http.Get(url)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	prefix := "Downloading " + label
	fmtBytesSize := 18
	barSize := int64(80 - len(prefix) - fmtBytesSize)
	bar := ioprogress.DrawTextFormatBar(barSize)
	fmtfunc := func(progress, total int64) string {
		return fmt.Sprintf(
			"%s: %s %s",
			prefix,
			bar(progress, total),
			ioprogress.DrawTextFormatBytes(progress, total),
		)
	}

	reader := &ioprogress.Reader{
		Reader:       res.Body,
		Size:         res.ContentLength,
		DrawFunc:     ioprogress.DrawTerminalf(os.Stdout, fmtfunc),
		DrawInterval: time.Second,
	}

	// TODO(jonboulle): handle http more robustly (redirects?)
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("bad HTTP status code: %d", res.StatusCode)
	}

	if _, err := io.Copy(output, reader); err != nil {
		return fmt.Errorf("error copying %s: %v", label, err)
	}

	if err := output.Sync(); err != nil {
		return fmt.Errorf("error writing %s: %v", label, err)
	}

	return nil
}
