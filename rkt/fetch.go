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

	"github.com/appc/spec/discovery"
	"github.com/appc/spec/schema/types"
)

const (
	defaultOS   = runtime.GOOS
	defaultArch = runtime.GOARCH
	cmdFetchName = "fetch"
)

var (
	cmdFetch = &Command{
		Name:    cmdFetchName,
		Summary: "Fetch image(s) and store them in the local cache",
		Usage:   "[--discovery-port PORT] IMAGE_URL...",
		Run:     runFetch,
	}
	flagDiscoveryPort uint
)

func init() {
	commands = append(commands, cmdFetch)
	cmdFetch.Flags.UintVar(&flagDiscoveryPort, "discovery-port", 0,
		"Port to connect when performing discovery. If unset or set to 0, defaults to standard HTTPS and HTTP ports")
}

func runFetch(args []string) (exit int) {
	if len(args) < 1 {
		printCommandUsageByName(cmdFetchName)
		return 1
	}

	ds := cas.NewStore(globalFlags.Dir)
	ks := getKeystore()
	for _, img := range args {
		hash, err := fetchImage(img, ds, ks)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			return 1
		}
		shortHash := types.ShortHash(hash)
		fmt.Println(shortHash)
	}

	return
}

// fetchImage will take an image as either a URL or a name string and import it
// into the store if found.
func fetchImage(img string, ds *cas.Store, ks *keystore.Keystore) (string, error) {
	u, err := url.Parse(img)
	if err == nil && u.Scheme == "" {
		if app := newDiscoveryApp(img); app != nil {
			fmt.Printf("rkt: starting to discover app img %s\n", img)
			ep, err := discovery.DiscoverEndpoints(*app, flagDiscoveryPort, true)
			if err != nil {
				return "", err
			}
			return fetchImageFromEndpoints(ep, ds, ks)
		}
	}
	if err != nil {
		return "", fmt.Errorf("not a valid URL (%s)", img)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("rkt only supports http or https URLs (%s)", img)
	}
	return fetchImageFromURL(u.String(), ds, ks)
}

func fetchImageFromEndpoints(ep *discovery.Endpoints, ds *cas.Store, ks *keystore.Keystore) (string, error) {
	rem := cas.NewRemote(ep.ACIEndpoints[0].ACI, ep.ACIEndpoints[0].Sig)
	return downloadImage(rem, ds, ks)
}

func fetchImageFromURL(imgurl string, ds *cas.Store, ks *keystore.Keystore) (string, error) {
	rem := cas.NewRemote(imgurl, sigURLFromImgURL(imgurl))
	return downloadImage(rem, ds, ks)
}

func downloadImage(rem *cas.Remote, ds *cas.Store, ks *keystore.Keystore) (string, error) {
	fmt.Printf("rkt: starting to fetch img from %s\n", rem.ACIURL)
	if globalFlags.InsecureSkipVerify {
		fmt.Printf("rkt: warning: signature verification has been disabled\n")
	}
	err := ds.ReadIndex(rem)
	if err != nil && rem.BlobKey == "" {
		entity, aciFile, err := rem.Download(*ds, ks)
		if err != nil {
			return "", err
		}
		defer os.Remove(aciFile.Name())

		if !globalFlags.InsecureSkipVerify {
			fmt.Println("rkt: signature verified signed by: ")
			for _, v := range entity.Identities {
				fmt.Printf("  %s\n", v.Name)
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
