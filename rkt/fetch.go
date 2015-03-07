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
		Usage:   "IMAGE_URL...",
		Run:     runFetch,
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

	ds, err := cas.NewStore(globalFlags.Dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fetch: cannot open store: %v\n", err)
		return 1
	}
	ks := getKeystore()

	for _, img := range args {
		hash, err := fetchImage(img, ds, ks, true)
		if err != nil {
			stderr("%v", err)
			return 1
		}
		shortHash := types.ShortHash(hash)
		fmt.Println(shortHash)
	}

	return
}

// fetchImage will take an image as either a URL or a name string and import it
// into the store if found.  If discover is true meta-discovery is enabled.
func fetchImage(img string, ds *cas.Store, ks *keystore.Keystore, discover bool) (string, error) {
	u, err := url.Parse(img)
	if err == nil && discover && u.Scheme == "" {
		if app := newDiscoveryApp(img); app != nil {
			stdout("rkt: searching for app image %s", img)
			ep, attempts, err := discovery.DiscoverEndpoints(*app, true)

			if globalFlags.Debug {
				for _, a := range attempts {
					stderr("meta tag 'ac-discovery' not found on %s: %v", a.Prefix, a.Error)
				}
			}

			if err != nil {
				return "", err
			}
			return fetchImageFromEndpoints(ep, ds, ks)
		}
	}
	if err != nil {
		return "", fmt.Errorf("not a valid URL (%s)", img)
	}
	switch u.Scheme {
	case "http", "https", "docker":
	default:
		return "", fmt.Errorf("rkt only supports http, https or docker URLs (%s)", img)
	}
	return fetchImageFromURL(u.String(), u.Scheme, ds, ks)
}

func fetchImageFromEndpoints(ep *discovery.Endpoints, ds *cas.Store, ks *keystore.Keystore) (string, error) {
	return downloadImage(ep.ACIEndpoints[0].ACI, ep.ACIEndpoints[0].ASC, "", ds, ks)
}

func fetchImageFromURL(imgurl string, scheme string, ds *cas.Store, ks *keystore.Keystore) (string, error) {
	return downloadImage(imgurl, sigURLFromImgURL(imgurl), scheme, ds, ks)
}

func downloadImage(aciURL string, sigURL string, scheme string, ds *cas.Store, ks *keystore.Keystore) (string, error) {
	stdout("rkt: fetching image from %s", aciURL)
	if globalFlags.InsecureSkipVerify {
		stdout("rkt: warning: signature verification has been disabled")
	} else if scheme == "docker" {
		return "", fmt.Errorf("signature verification for docker images is not supported (try --insecure-skip-verify)")
	}
	rem, ok, err := ds.GetRemote(aciURL)
	if err != nil {
		return "", err
	}
	if !ok {
		rem = cas.NewRemote(aciURL, sigURL)
		entity, aciFile, err := rem.Download(*ds, ks)
		if err != nil {
			return "", err
		}
		defer os.Remove(aciFile.Name())

		if entity != nil && !globalFlags.InsecureSkipVerify {
			fmt.Println("rkt: signature verified: ")
			for _, v := range entity.Identities {
				stdout("  %s", v.Name)
			}
		}
		rem, err = rem.Store(*ds, aciFile)
		if err != nil {
			return "", err
		}
	}
	return rem.BlobKey, nil
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
