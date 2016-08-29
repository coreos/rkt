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

package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/coreos/rkt/api/v1alpha"
	"github.com/coreos/rkt/common/apps"
	"github.com/coreos/rkt/pkg/keystore"
	"github.com/coreos/rkt/rkt/config"
	rktflag "github.com/coreos/rkt/rkt/flag"
	"github.com/coreos/rkt/rkt/image"
	"github.com/coreos/rkt/store/imagestore"
	"github.com/coreos/rkt/store/treestore"
	"golang.org/x/net/context"
)

// v1alphaAPIReadOnlyServer implements v1alpha.PublicWriteOnlyAPI interface.
type v1alphaWriteOnlyAPIServer struct {
	store     *imagestore.Store
	treeStore *treestore.Store
	keyStore  *keystore.Keystore
}

var _ v1alpha.PublicWriteOnlyAPIServer = &v1alphaWriteOnlyAPIServer{}

func newV1alphaWriteOnlyAPIServer(s *imagestore.Store) (*v1alphaWriteOnlyAPIServer, error) {
	ts, err := treestore.NewStore(treeStoreDir(), s)
	if err != nil {
		stderr.PrintE("cannot open treestore", err)
		return nil, err
	}

	return &v1alphaWriteOnlyAPIServer{
		store:     s,
		treeStore: ts,
		keyStore:  getKeystore(),
	}, nil
}

func (s *v1alphaWriteOnlyAPIServer) FetchImages(ctx context.Context, request *v1alpha.FetchImagesRequest) (*v1alpha.FetchImagesResponse, error) {
	var errlist []error
	var cfg config.Config
	var ids []string

	insecureOption, err := getInsecureOption(request.InsecureOption)
	if err != nil {
		stderr.PrintE(fmt.Sprintf("invalid insecure option: %q", request.InsecureOption), err)
		return nil, err
	}

	if len(request.CredentialConfig) > 0 {
		if err := json.Unmarshal(request.CredentialConfig, &cfg); err != nil {
			stderr.PrintE("failed to unmarshal credential config", err)
			return nil, err
		}
	}

	ft := &image.Fetcher{
		S:                  s.store,
		Ts:                 s.treeStore,
		Ks:                 s.keyStore,
		Headers:            cfg.AuthPerHost,
		DockerAuth:         cfg.DockerCredentialsPerRegistry,
		InsecureFlags:      insecureOption,
		TrustKeysFromHTTPS: request.TrustKeysFromHttps,

		StoreOnly: request.DiscoverOption == v1alpha.DiscoverOption_DISCOVER_OPTION_ONLY_STORE,
		NoStore:   request.DiscoverOption == v1alpha.DiscoverOption_DISCOVER_OPTION_NO_STORE,
		WithDeps:  request.WithDeps,
	}

	for _, name := range request.Names {
		// TODO(yifan): Embed asc file paths/or data in the request?
		// TODO(yifan): Parse the distribution string.
		hash, err := ft.FetchImage(name, "", apps.AppImageGuess)
		if err != nil {
			stderr.PrintE(fmt.Sprintf("failed to fetch image %q", name), err)
			errlist = append(errlist, err)
		} else {
			ids = append(ids, hash)
		}
	}

	if len(errlist) > 0 {
		return nil, errs{errlist}
	}
	return &v1alpha.FetchImagesResponse{Ids: ids}, nil
}

func getInsecureOption(opt v1alpha.InsecureOption) (*rktflag.SecFlags, error) {
	var optlist []string

	if opt == v1alpha.InsecureOption_INSECURE_OPTION_NONE {
		optlist = append(optlist, "none")
	}
	if opt&v1alpha.InsecureOption_INSECURE_OPTION_ALL > 0 {
		optlist = append(optlist, "all")
	}
	if opt&v1alpha.InsecureOption_INSECURE_OPTION_IMAGE > 0 {
		optlist = append(optlist, "image")
	}
	if opt&v1alpha.InsecureOption_INSECURE_OPTION_TLS > 0 {
		optlist = append(optlist, "tls")
	}
	if opt&v1alpha.InsecureOption_INSECURE_OPTION_ON_DISK > 0 {
		optlist = append(optlist, "ondisk")
	}
	if opt&v1alpha.InsecureOption_INSECURE_OPTION_HTTP > 0 {
		optlist = append(optlist, "http")
	}
	if opt&v1alpha.InsecureOption_INSECURE_OPTION_PUBKEY > 0 {
		optlist = append(optlist, "pubkey")
	}

	return rktflag.NewSecFlags(strings.Join(optlist, ","))
}

func (s *v1alphaWriteOnlyAPIServer) RemoveImages(ctx context.Context, request *v1alpha.RemoveImagesRequest) (*v1alpha.RemoveImagesResponse, error) {
	var errlist []error
	var ids []string

	if len(request.Ids) > 0 {
		ids = request.Ids
	}

	if len(request.Names) > 0 {
		aciInfos, err := s.store.GetAllACIInfos(nil, false)
		if err != nil {
			stderr.PrintE(fmt.Sprintf("failed to remove images %q", request.Names), err)
			errlist = append(errlist, err)
		} else {
			for _, name := range request.Names {
				for _, img := range aciInfos {
					if name == img.Name {
						ids = append(ids, img.BlobKey)
					}
				}
			}
		}
	}

	if err := rmImages(s.store, ids); err != nil {
		stderr.PrintE(fmt.Sprintf("failed to remove images %q", ids), err)
		errlist = append(errlist, err)
	}

	if len(errlist) > 0 {
		return nil, errs{errlist}
	}
	return &v1alpha.RemoveImagesResponse{}, nil
}
