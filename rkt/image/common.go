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
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	dist "github.com/coreos/rkt/common/distribution"
	"github.com/coreos/rkt/pkg/keystore"
	rktlog "github.com/coreos/rkt/pkg/log"
	"github.com/coreos/rkt/rkt/config"
	rktflag "github.com/coreos/rkt/rkt/flag"
	"github.com/coreos/rkt/store/casref/rwcasref"
	"github.com/coreos/rkt/store/manifestcache"
	"github.com/coreos/rkt/store/treestore"
	"github.com/hashicorp/errwrap"

	"github.com/appc/spec/discovery"
	"github.com/appc/spec/schema"
	"github.com/appc/spec/schema/types"
	"golang.org/x/crypto/openpgp"
)

type imageStringType int

const (
	imageStringName imageStringType = iota // image type to be guessed
	imageStringPath                        // absolute or relative path
)

// action is a common type for Finder and Fetcher
type action struct {
	// S is an aci store where images will be looked for or stored.
	S *rwcasref.Store
	// Ts is an aci tree store.
	Ts *treestore.Store
	// Mc is an aci manifest cache.
	Mc *manifestcache.ACIManifestCache
	// Ks is a keystore used for verification of the image
	Ks *keystore.Keystore
	// Headers is a map of headers which might be used for
	// downloading via https protocol.
	Headers map[string]config.Headerer
	// DockerAuth is used for authenticating when fetching docker
	// images.
	DockerAuth map[string]config.BasicCredentials
	// InsecureFlags is a set of flags for enabling some insecure
	// functionality. For now it is mostly skipping image
	// signature verification and TLS certificate verification.
	InsecureFlags *rktflag.SecFlags
	// Debug tells whether additional debug messages should be
	// printed.
	Debug bool
	// TrustKeysFromHTTPS tells whether discovered keys downloaded
	// via the https protocol can be trusted
	TrustKeysFromHTTPS bool

	// StoreOnly tells whether to avoid getting images from a
	// local filesystem or a remote location.
	StoreOnly bool
	// NoStore tells whether to avoid getting images from the
	// store. Note that transport caching (like http Etags) can be still
	// used to avoid refetching.
	NoStore bool
	// NoCache tells to ignore transport caching.
	NoCache bool
	// WithDeps tells whether image dependencies should be
	// downloaded too.
	WithDeps bool
}

var (
	log    *rktlog.Logger
	stdout *rktlog.Logger = rktlog.New(os.Stdout, "", false)
)

func ensureLogger(debug bool) {
	if log == nil {
		log = rktlog.New(os.Stderr, "image", debug)
	}
}

// isReallyNil makes sure that the passed value is really really
// nil. So it returns true if value is plain nil or if it is e.g. an
// interface with non-nil type but nil-value (which normally is
// different from nil itself).
func isReallyNil(iface interface{}) bool {
	// this catches the cases when you pass non-interface nil
	// directly, like:
	//
	// isReallyNil(nil)
	// var m map[string]string
	// isReallyNil(m)
	if iface == nil {
		return true
	}
	// get a reflect value
	v := reflect.ValueOf(iface)
	// only channels, functions, interfaces, maps, pointers and
	// slices are nillable
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		// this catches the cases when you pass some interface
		// with nil value, like:
		//
		// var v io.Closer = func(){var f *os.File; return f}()
		// isReallyNil(v)
		return v.IsNil()
	}
	return false
}

// useCached checks if downloadTime plus maxAge is before/after the current time.
// return true if the cached image should be used, false otherwise.
func useCached(downloadTime time.Time, maxAge int) bool {
	freshnessLifetime := int(time.Now().Sub(downloadTime).Seconds())
	if maxAge > 0 && freshnessLifetime < maxAge {
		return true
	}
	return false
}

// ascURLFromImgURL creates a URL to a signature file from passed URL
// to an image.
func ascURLFromImgURL(u *url.URL) *url.URL {
	copy := *u
	copy.Path = ascPathFromImgPath(copy.Path)
	return &copy
}

// ascPathFromImgPath creates a path to a signature file from passed
// path to an image.
func ascPathFromImgPath(path string) string {
	return fmt.Sprintf("%s.aci.asc", strings.TrimSuffix(path, ".aci"))
}

// printIdentities prints a message that signature was verified.
func printIdentities(entity *openpgp.Entity) {
	lines := []string{"signature verified:"}
	for _, v := range entity.Identities {
		lines = append(lines, fmt.Sprintf("  %s", v.Name))
	}
	log.Print(strings.Join(lines, "\n"))
}

// DistFromImageString return the distribution for the given input image string
func DistFromImageString(is string) (dist.Distribution, error) {
	// Guess if an user friendly image string is an appc string or a file
	if u, err := url.Parse(is); err == nil && u.Scheme == "" {
		appImageType := guessAppcOrPath(is, []string{schema.ACIExtension})
		switch appImageType {
		case imageStringName:
			app, err := discovery.NewAppFromString(is)
			if err != nil {
				return nil, fmt.Errorf("bad appc image string %q: %v", is, err)
			}
			return dist.NewAppcFromApp(app), nil
		case imageStringPath:
			absPath, err := filepath.Abs(is)
			if err != nil {
				return nil, errwrap.Wrap(fmt.Errorf("failed to get an absolute path for %q", is), err)
			}
			is = "file://" + absPath
		default:
			return nil, fmt.Errorf("wrong image string type")
		}
	}

	// Now convert the user friendly URLs to a distribution
	if u, err := url.Parse(is); err == nil && u.Scheme != "" {
		// Convert user friendly image string names to internal distribution URIs
		// file:///full/path/to/aci/file.aci -> archive:aci:file%3A%2F%2F%2Ffull%2Fpath%2Fto%2Faci%2Ffile.aci
		switch u.Scheme {
		case "file", "http", "https":
			// An ACI archive with any transport type (file, http, s3 etc...) and final aci extension
			if filepath.Ext(u.Path) == schema.ACIExtension {
				dist, err := dist.NewACIArchiveFromURL(u)
				if err != nil {
					return nil, fmt.Errorf("archive distribution creation error: %v", err)
				}
				return dist, nil
			}
		case "docker":
			// Accept both docker: and docker:// uri
			dockerStr := is
			if strings.HasPrefix(dockerStr, "docker://") {
				dockerStr = strings.TrimPrefix(dockerStr, "docker://")
			} else if strings.HasPrefix(dockerStr, "docker:") {
				dockerStr = strings.TrimPrefix(dockerStr, "docker:")
			}

			dist, err := dist.NewDockerFromDockerString(dockerStr)
			if err != nil {
				return nil, fmt.Errorf("docker distribution creation error: %v", err)
			}
			return dist, nil
		default:
			// Try to create a dist directly from the provided URI
			return dist.NewDistribution(is)

		}
	}
	return nil, fmt.Errorf("bad image string %q", is)
}

func guessAppcOrPath(is string, extensions []string) imageStringType {
	if filepath.IsAbs(is) {
		return imageStringPath
	}

	// Well, at this point is basically heuristics time. The image
	// parameter can be either a relative path or an image name.

	// First, let's try to stat whatever file the URL would specify. If it
	// exists, that's probably what the user wanted.
	f, err := os.Stat(is)
	if err == nil && f.Mode().IsRegular() {
		return imageStringPath
	}

	// Second, let's check if there is a colon in the image
	// parameter. Colon often serves as a paths separator (like in
	// the PATH environment variable), so if it exists, then it is
	// highly unlikely that the image parameter is a path. Colon
	// in this context is often used for specifying a version of
	// an image, like in "example.com/reduce-worker:1.0.0".
	if strings.ContainsRune(is, ':') {
		return imageStringName
	}

	// Third, let's check if there is a dot followed by a slash
	// (./) - if so, it is likely that the image parameter is path
	// like ./aci-in-this-dir or ../aci-in-parent-dir
	if strings.Contains(is, "./") {
		return imageStringPath
	}

	// Fourth, let's check if the image parameter has an .aci
	// extension. If so, likely a path like "stage1-coreos.aci".
	for _, e := range extensions {
		if filepath.Ext(is) == e {
			return imageStringPath
		}
	}

	// At this point, if the image parameter is something like
	// "coreos.com/rkt/stage1-coreos" and you have a directory
	// tree "coreos.com/rkt" in the current working directory and
	// you meant the image parameter to point to the file
	// "stage1-coreos" in this directory tree, then you better be
	// off prepending the parameter with "./", because I'm gonna
	// treat this as an image name otherwise.
	return imageStringName
}

// DistSimpleString returns an user friendly string representation of the distribution
func DistSimpleString(d dist.Distribution) string {
	switch d.Type() {
	case dist.DistTypeAppc:
		return AppcSimpleString(d.(*dist.Appc))
	case dist.DistTypeACIArchive:
		return d.(*dist.ACIArchive).ArchiveURL().String()
	case dist.DistTypeDocker:
		return "docker:" + d.(*dist.Docker).SimpleDockerString()
	default:
		return d.ComparableURIString()
	}
}

// AppcSimpleString returns the simplest appc image format string from the Appc
// distribution
// To avoid random label position they are alphabetically ordered giving the
// precedence to version, os, arch (in this order)
func AppcSimpleString(a *dist.Appc) string {
	app := a.App()
	var version string
	labels := types.Labels{}
	for n, v := range app.Labels {
		if n == "version" {
			version = v
		} else {
			labels = append(labels, types.Label{Name: n, Value: v})
		}
	}
	sort.Sort(labelsSlice(labels))

	img := app.Name.String()
	if version != "" {
		img += fmt.Sprintf(":%s", version)
	}
	for _, l := range labels {
		if l.Name != "version" {
			img += fmt.Sprintf(",%s=%s", l.Name, l.Value)
		}
	}
	return img
}
