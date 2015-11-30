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

//+build linux

package main

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/schema/types"
	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/spf13/cobra"
	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/spf13/pflag"
	"github.com/coreos/rkt/store"
)

const (
	stage1ImageFlagName string = "stage1-image"
)

var (
	defaultStage1Image   string // set by linker
	defaultStage1Name    string // set by linker
	defaultStage1Version string // set by linker
)

// addStage1ImageFlag adds a flag for specifying custom stage1 image
// path
func addStage1ImageFlag(flags *pflag.FlagSet) {
	flags.String(stage1ImageFlagName, defaultStage1Image, stage1ImageFlagHelp())
}

func stage1ImageFlagHelp() string {
	u, err := url.Parse(defaultStage1Image)
	if err != nil {
		panic(fmt.Sprintf("defaultStage1Image %q is not a valid URL: %v", defaultStage1Image, err))
	}
	imgDir := filepath.Dir(defaultStage1Image)
	imgBase := filepath.Base(defaultStage1Image)
	firstPart := "image to use as stage1. Local paths and http/https URLs are supported"

	if len(defaultStage1Image) < 1 {
		return fmt.Sprintf(`%s. There is no default stage1 image to use`, firstPart)
	}
	if u.Scheme != "" {
		return fmt.Sprintf(`%s. By default, rkt will look for a file %q`, firstPart, defaultStage1Image)
	}
	if filepath.IsAbs(imgDir) {
		return fmt.Sprintf(`%s. By default, rkt will look for a file called %q first in %q directory, then in the same directory as rkt itself`, firstPart, imgBase, imgDir)
	}
	return fmt.Sprintf(`%s. By default, rkt will look for a file called %q in the same directory as rkt itself`, firstPart, imgBase)
}

// getStage1Hash will try to fetch stage1 from store if it is a
// default one. If that fails it will try to get via usual fetching
// from disk or network.
//
// As a special case, if stage1 image path is a default and not
// overriden by --stage1-image flag and it is has no scheme, it will
// try to fetch it from two places on disk - from the path directly if
// it is absolute and then from the same directory where rkt binary
// resides.
//
// The passed command must have "stage1-image" string flag registered.
func getStage1Hash(s *store.Store, cmd *cobra.Command) (*types.Hash, error) {
	fn := &finder{
		imageActionData: imageActionData{
			s:             s,
			insecureFlags: globalFlags.InsecureFlags,
		},
		storeOnly: false,
		noStore:   false,
		withDeps:  false,
	}

	imageFlag := cmd.Flags().Lookup(stage1ImageFlagName)
	if imageFlag == nil {
		panic(fmt.Sprintf("Expected flag --%s to be registered in command %s", stage1ImageFlagName, cmd.Name()))
	}
	path := imageFlag.Value.String()
	if path == defaultStage1Image {
		return getDefaultStage1Hash(fn, imageFlag.Changed)
	}
	return getCustomStage1Hash(fn, path)
}

func getDefaultStage1Hash(fn *finder, overridden bool) (*types.Hash, error) {
	// just fail if default stage1 image path is empty - might
	// happen for none flavor
	if defaultStage1Image == "" {
		return nil, fmt.Errorf("no default stage1 image is set, use --%s flag", stage1ImageFlagName)
	}

	if s1img := getDefaultStage1HashFromStore(fn); s1img != nil {
		return s1img, nil
	}
	// we couldn't find a proper stage1 image in the store, fall
	// back to using a stage1 ACI file (slow)
	return getDefaultStage1HashFromUrl(fn, overridden)
}

func getDefaultStage1HashFromStore(fn *finder) *types.Hash {
	// we make sure we've built rkt with a clean git tree,
	// otherwise we don't know if something changed
	if !strings.HasSuffix(defaultStage1Version, "-dirty") {
		stage1AppName := fmt.Sprintf("%s:%s", defaultStage1Name, defaultStage1Version)
		fn.storeOnly = true
		s1img, _ := fn.findImage(stage1AppName, "")
		fn.storeOnly = false
		return s1img
	}
	return nil
}

func getDefaultStage1HashFromUrl(fn *finder, overridden bool) (*types.Hash, error) {
	if overridden {
		// we specified --stage1-image parameter explicitly,
		// so just try to get it without further or more ado
		return getCustomStage1Hash(fn, defaultStage1Image)
	}
	if url, err := url.Parse(defaultStage1Image); err != nil {
		return nil, fmt.Errorf("failed to parse %q as URL: %v", defaultStage1Image, err)
	} else if url.Scheme != "" {
		// default stage1 image path is some schemed path,
		// just try to get it
		getCustomStage1Hash(fn, defaultStage1Image)
	}
	// default scheme path is a relative or absolute path to a
	// file
	return getLocalDefaultStage1Hash(fn)
}

func getLocalDefaultStage1Hash(fn *finder) (*types.Hash, error) {
	var firstErr error = nil
	// Guard against relative path to default stage1 image. It
	// could be fine for some custom stage1 path passed via
	// --stage1-image parameter, but not for default, not
	// overridden ones. Usually, if the path is relative, then it
	// means we simply didn't pass --with-stage1-image-path
	// parameter to configure script, so it defaulted to desired
	// filename like "stuff/yadda/stage1-lkvm.aci" (or something).
	if filepath.IsAbs(defaultStage1Image) {
		s1img, err := fn.findImage(defaultStage1Image, "")
		if s1img != nil {
			return s1img, nil
		}
		firstErr = err
	}
	// could not find default stage1 image in a given path
	// (or we ignored it), lets try to load one in the
	// same directory as rkt itself
	imgBase := filepath.Base(defaultStage1Image)
	exePath, err := os.Readlink("/proc/self/exe")
	if err != nil {
		if firstErr != nil {
			return nil, fmt.Errorf("error finding stage1 images %q and %q in rkt binary directory: %v and %v", defaultStage1Image, imgBase, firstErr, err)
		} else {
			return nil, fmt.Errorf("error finding stage1 image %q in rkt binary directory: %v", imgBase, err)
		}
	}
	fallbackPath := filepath.Join(filepath.Dir(exePath), imgBase)
	s1img, err := fn.findImage(fallbackPath, "")
	if err != nil {
		if firstErr != nil {
			return nil, fmt.Errorf("error finding stage1 images %q and %q: %v and %v", defaultStage1Image, fallbackPath, firstErr, err)
		} else {
			return nil, fmt.Errorf("error finding stage1 image %q: %v", fallbackPath, err)
		}
	}
	return s1img, nil
}

func getCustomStage1Hash(fn *finder, path string) (*types.Hash, error) {
	s1img, err := fn.findImage(path, "")
	if err != nil {
		return nil, fmt.Errorf("error finding stage1 image %q: %v", path, err)
	}
	return s1img, nil
}
