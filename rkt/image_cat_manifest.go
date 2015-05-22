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
	"flag"

	"github.com/coreos/rkt/store"
)

var (
	cmdImageCatManifest = &Command{
		Name:    "cat-manifest",
		Summary: "Inspect and print the image manifest",
		Usage:   "IMAGE",
		Description: `IMAGE should be a string referencing an image; either a hash, local file on disk, or URL.
They will be checked in that order and the first match will be used.`,
		Run:   runImageCatManifest,
		Flags: &imageCatManifestFlag,
	}
	imageCatManifestFlag flag.FlagSet
	flagPrettyPrint      bool
)

func init() {
	subCommands["image"] = append(subCommands["image"], cmdImageCatManifest)

	imageCatManifestFlag.BoolVar(&flagPrettyPrint, "pretty-print", false, "apply indent to format the output")
}

func runImageCatManifest(args []string) (exit int) {
	if len(args) != 1 {
		printSubCommandUsageByName("image", "cat-manifest", subCommands["image"])
		return 1
	}

	s, err := store.NewStore(globalFlags.Dir)
	if err != nil {
		stderr("image cat-manifest: cannot open store: %v\n", err)
		return 1
	}
	ks := getKeystore()

	fn := &finder{
		imageActionData: imageActionData{
			s:                  s,
			ks:                 ks,
			insecureSkipVerify: true,
			debug:              globalFlags.Debug,
		},
		local:    false,
		withDeps: false,
	}

	h, err := fn.findImage(args[0], "", true)
	if err != nil {
		stderr("image cat-manifest: cannot find image: %v\n", err)
		return 1
	}

	manifest, err := fn.s.GetImageManifest(h.String())
	if err != nil {
		stderr("image cat-manifest: cannot get image manifest: %v\n", err)
		return 1
	}

	var b []byte
	if flagPrettyPrint {
		b, err = json.MarshalIndent(manifest, "", "\t")
	} else {
		b, err = json.Marshal(manifest)
	}
	if err != nil {
		stderr("image cat-manifest: cannot read the image manifest: %v\n", err)
		return 1
	}

	stdout(string(b))
	return 0
}
