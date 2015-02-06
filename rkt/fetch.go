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

package main

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/coreos/rocket/cas"
	"github.com/coreos/rocket/pkg/keystore"

	"github.com/coreos/rocket/Godeps/_workspace/src/github.com/appc/spec/discovery"
	"github.com/coreos/rocket/Godeps/_workspace/src/github.com/appc/spec/schema/types"
)

const (
	defaultOS   = runtime.GOOS
	defaultArch = runtime.GOARCH
)

var (
	cmdFetch = &Command{
		Name:    "fetch",
		Summary: "Fetch image(s) and store them in the local cache",
		Description: `IMAGE may be specified as an:
  Local file e.g. "/tmp/foo.aci" or "foo.aci", or an
  URL of http://, https://, or file:// scheme.

Once cached the images may be referenced directly by the hashes printed.`,
		Usage: "IMAGE...",
		Run:   runFetch,
	}
)

func init() {
	commands = append(commands, cmdFetch)
}

func runFetch(args []string) (exit int) {
	if len(args) < 1 {
		stderr("fetch: Must provide at least one image")
		return 1
	}

	ds := cas.NewStore(globalFlags.Dir)
	ks := getKeystore()
	for _, img := range args {
		hash, err := fetchImage(img, ds, ks, true)
		if err != nil {
			stderr("fetch error: %v", err)
			return 1
		}
		shortHash := types.ShortHash(hash)
		fmt.Println(shortHash)
	}

	return
}

// fetchImage will take an image as either a existing local file path, URL (file|http|https),
// or a name string and import it into the store if found.
// If discover is true meta-discovery is enabled for name string resolution.
func fetchImage(img string, ds *cas.Store, ks *keystore.Keystore, discover bool) (string, error) {
	// normalize to a file:// URL if img exists as a local file
	_, err := os.Stat(img)
	if err == nil {
		path, err := filepath.Abs(img)
		if err != nil {
			return "", fmt.Errorf("error creating absolute path: %v", err)
		}
		img = fmt.Sprintf("file://%s", filepath.Clean(path))
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("unable to access local file: %v", err)
	}

	u, err := url.Parse(img)
	if err == nil && discover && u.Scheme == "" {
		if app := newDiscoveryApp(img); app != nil {
			stdout("rkt: searching for app image %s", img)
			ep, err := discovery.DiscoverEndpoints(*app, true)
			if err != nil {
				return "", err
			}
			return fetchImageFromEndpoints(ep, ds, ks)
		}
	}
	if err != nil {
		return "", fmt.Errorf("not a valid URL (%s)", img)
	}
	if u.Scheme != "file" && u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("rkt only supports http or https URLs (%s)", img)
	}
	return fetchImageFromURL(u.String(), ds, ks)
}

func fetchImageFromEndpoints(ep *discovery.Endpoints, ds *cas.Store, ks *keystore.Keystore) (string, error) {
	fr := cas.NewFetcher(ep.ACIEndpoints[0].ACI, ep.ACIEndpoints[0].Sig)
	return getImage(fr, ds, ks)
}

func fetchImageFromURL(imgurl string, ds *cas.Store, ks *keystore.Keystore) (string, error) {
	fr := cas.NewFetcher(imgurl, sigURLFromImgURL(imgurl))
	return getImage(fr, ds, ks)
}

func getImage(fr *cas.Fetcher, ds *cas.Store, ks *keystore.Keystore) (string, error) {
	if globalFlags.Debug {
		stdout("rkt: fetching image from %s", fr.ACIURL)
	}
	if globalFlags.InsecureSkipVerify {
		stdout("rkt: warning: signature verification has been disabled")
	}
	err := ds.ReadIndex(fr)
	if err != nil && fr.BlobKey == "" {
		entity, aciFile, isRemote, err := fr.Get(*ds, ks)
		if err != nil {
			return "", err
		}
		if isRemote {
			defer os.Remove(aciFile.Name())
		}

		if !globalFlags.InsecureSkipVerify {
			fmt.Println("rkt: signature verified: ")
			for _, v := range entity.Identities {
				stdout("  %s", v.Name)
			}
		}
		fr, err = fr.Store(*ds, aciFile)
		if err != nil {
			return "", err
		}
	}
	return fr.BlobKey, nil
}

func validateURL(s string) error {
	u, err := url.Parse(s)
	if err != nil {
		return fmt.Errorf("discovery: fetched URL (%s) is invalid (%v)", s, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("rkt only supports http or https URLs (%s)", s)
	}
	return nil
}

func sigURLFromImgURL(imgurl string) string {
	s := strings.TrimSuffix(imgurl, ".aci")
	return s + ".sig"
}

// newDiscoveryApp creates a discovery app if the given img is an app name and
// has a URL-like structure, for example example.com/reduce-worker.
// Or it returns nil.
func newDiscoveryApp(img string) *discovery.App {
	app, err := discovery.NewAppFromString(img)
	if err != nil {
		return nil
	}
	u, err := url.Parse(app.Name.String())
	if err != nil || u.Scheme != "" {
		return nil
	}
	if _, ok := app.Labels["arch"]; !ok {
		app.Labels["arch"] = defaultArch
	}
	if _, ok := app.Labels["os"]; !ok {
		app.Labels["os"] = defaultOS
	}
	return app
}
