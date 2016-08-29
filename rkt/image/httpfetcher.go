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
	"errors"
	"io"
	"net/url"

	"github.com/coreos/rkt/common/image/aci"
	"github.com/coreos/rkt/pkg/keystore"
	"github.com/coreos/rkt/rkt/config"
	rktflag "github.com/coreos/rkt/rkt/flag"
	"github.com/coreos/rkt/store/casref/rwcasref"
	"github.com/hashicorp/errwrap"
)

// httpFetcher is used to download images from http or https URLs.
type httpFetcher struct {
	InsecureFlags *rktflag.SecFlags
	S             *rwcasref.Store
	Ks            *keystore.Keystore
	NoCache       bool
	Debug         bool
	Headers       map[string]config.Headerer
}

// Hash fetches the URL, optionally verifies it against passed asc,
// stores it in the store and returns the hash.
func (f *httpFetcher) Hash(u *url.URL, a *asc) (string, error) {
	ensureLogger(f.Debug)
	urlStr := u.String()

	//if !f.NoCache && f.Rem != nil {
	//	if useCached(f.Rem.DownloadTime, f.Rem.CacheMaxAge) {
	//		if f.Debug {
	//			log.Printf("image for %s isn't expired, not fetching.", urlStr)
	//		}
	//		return f.Rem.BlobKey, nil
	//	}
	//}

	if f.Debug {
		log.Printf("fetching image from %s", urlStr)
	}

	aciFile, _, err := f.fetchURL(u, a, "")
	if err != nil {
		return "", err
	}
	defer func() { maybeClose(aciFile) }()
	//if key := f.maybeUseCached(cd); key != "" {
	//	// TODO(krnowak): that does not update the store with
	//	// the new CacheMaxAge and Download Time, so it will
	//	// query the server every time after initial
	//	// CacheMaxAge is exceeded
	//	return key, nil
	//}
	digest, err := aci.WriteACI(f.S, aciFile)
	if err != nil {
		return "", err
	}
	return digest, nil
}

func (f *httpFetcher) fetchURL(u *url.URL, a *asc, etag string) (readSeekCloser, *cacheData, error) {
	if f.InsecureFlags.SkipTLSCheck() && f.Ks != nil {
		log.Printf("warning: TLS verification has been disabled")
	}
	if f.InsecureFlags.SkipImageCheck() && f.Ks != nil {
		log.Printf("warning: image signature verification has been disabled")
	}

	if f.InsecureFlags.SkipImageCheck() || f.Ks == nil {
		o := f.httpOps()
		aciFile, cd, err := o.DownloadImageWithETag(u, etag)
		if err != nil {
			return nil, nil, err
		}
		return aciFile, cd, err
	}

	return f.fetchVerifiedURL(u, a, etag)
}

func (f *httpFetcher) fetchVerifiedURL(u *url.URL, a *asc, etag string) (readSeekCloser, *cacheData, error) {
	o := f.httpOps()
	f.maybeOverrideAscFetcherWithRemote(o, u, a)
	ascFile, retry, err := o.DownloadSignature(a)
	if err != nil {
		return nil, nil, err
	}
	defer func() { maybeClose(ascFile) }()

	aciFile, cd, err := o.DownloadImageWithETag(u, etag)
	if err != nil {
		return nil, nil, err
	}
	defer func() { maybeClose(aciFile) }()
	if cd.UseCached {
		return nil, cd, nil
	}

	if retry {
		ascFile, err = o.DownloadSignatureAgain(a)
		if err != nil {
			return nil, nil, err
		}
	}

	if err := f.validate(aciFile, ascFile); err != nil {
		return nil, nil, err
	}
	retAciFile := aciFile
	aciFile = nil
	return retAciFile, cd, nil
}

func (f *httpFetcher) httpOps() *httpOps {
	return &httpOps{
		InsecureSkipTLSVerify: f.InsecureFlags.SkipTLSCheck(),
		S:       f.S,
		Headers: f.Headers,
		Debug:   f.Debug,
	}
}

func (f *httpFetcher) validate(aciFile, ascFile io.ReadSeeker) error {
	v, err := newValidator(aciFile)
	if err != nil {
		return err
	}
	entity, err := v.ValidateWithSignature(f.Ks, ascFile)
	if err != nil {
		return err
	}
	if _, err := aciFile.Seek(0, 0); err != nil {
		return errwrap.Wrap(errors.New("error seeking ACI file"), err)
	}

	printIdentities(entity)
	return nil
}

func (f *httpFetcher) maybeOverrideAscFetcherWithRemote(o *httpOps, u *url.URL, a *asc) {
	if a.Fetcher != nil {
		return
	}
	u2 := ascURLFromImgURL(u)
	a.Location = u2.String()
	a.Fetcher = o.AscRemoteFetcher()
}
