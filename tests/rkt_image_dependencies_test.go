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
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/coreos/rkt/tests/testutils"
)

const (
	manifestDepsTemplate = `
{
   "acKind" : "ImageManifest",
   "acVersion" : "0.7.3",
   "dependencies" : [
      DEPENDENCIES
   ],
   "labels" : [
      {
         "name" : "version",
         "value" : "1.0.0"
      },
      {
         "name" : "arch",
         "value" : "amd64"
      },
      {
         "value" : "linux",
         "name" : "os"
      }
   ],
   "app" : {
      "user" : "0",
      "exec" : [
         "/inspect", "--print-msg=HelloDependencies"
      ],
      "workingDirectory" : "/",
      "group" : "0",
      "environment" : [
      ]
   },
   "name" : "IMG_NAME"
}
`
)

// TestImageRender tests 'rkt image render', it will import some existing empty
// image with a dependency on an image with the inspect binary, render it with
// rkt image render and check that the exported image has the /inspect file and
// that its hash matches the original inspect binary hash
func TestImageDependencies(t *testing.T) {
	tmpDir := createTempDirOrPanic("rkt-TestImageDeps-")
	defer os.RemoveAll(tmpDir)

	ctx := testutils.NewRktRunCtx()
	defer ctx.Cleanup()

	baseImage := getInspectImagePath()
	_ = importImageAndFetchHash(t, ctx, baseImage)
	emptyImage := getEmptyImagePath()

	// Dependencies:
	//
	// A --> B --> C --> D --> inspect
	//  \          ^
	//   \         |
	//    ---------/

	topImage := "localhost/image-a"
	imageList := []struct {
		shortName string
		imageName string
		deps      string

		manifest string
		fileName string
	}{
		{
			shortName: "d",
			imageName: "localhost/image-d",
			deps:      `{"imageName":"coreos.com/rkt-inspect"}`,
		},
		{
			shortName: "c",
			imageName: "localhost/image-c",
			deps:      `{"imageName":"localhost/image-d"}`,
		},
		{
			shortName: "b",
			imageName: "localhost/image-b",
			deps:      `{"imageName":"localhost/image-c"}`,
		},
		{
			shortName: "a",
			imageName: topImage,
			deps:      `{"imageName":"localhost/image-b"}, {"imageName":"localhost/image-c"}`,
		},
	}

	for _, img := range imageList {
		img.manifest = manifestDepsTemplate
		img.manifest = strings.Replace(img.manifest, "IMG_NAME", img.imageName, -1)
		img.manifest = strings.Replace(img.manifest, "DEPENDENCIES", img.deps, -1)

		tmpManifest, err := ioutil.TempFile(tmpDir, "manifest-"+img.shortName+"-")
		if err != nil {
			panic(fmt.Sprintf("Cannot create temp manifest: %v", err))
		}
		if err := ioutil.WriteFile(tmpManifest.Name(), []byte(img.manifest), 0600); err != nil {
			panic(fmt.Sprintf("Cannot write to temp manifest: %v", err))
		}
		defer os.Remove(tmpManifest.Name())

		img.fileName = patchACI(emptyImage, "image-"+img.shortName+".aci", "--manifest", tmpManifest.Name())
		defer os.Remove(img.fileName)

		// We cannot test real discovery for now
		// https://github.com/coreos/rkt/pull/375#issuecomment-160901072
		// So we import the images into the CAS.

		testImageShortHash := importImageAndFetchHash(t, ctx, img.fileName)
		t.Logf("Imported image %q: %s", img.imageName, testImageShortHash)
	}

	runCmd := fmt.Sprintf("%s --debug run %s", ctx.Cmd(), "localhost/image-a")
	child := spawnOrFail(t, runCmd)

	expectedList := []string{
		"rkt: using image from local store for image name localhost/image-a",
		"rkt: using image from local store for image name localhost/image-b",
		"rkt: using image from local store for image name localhost/image-c",
		"rkt: using image from local store for image name localhost/image-d",
		"rkt: using image from local store for image name coreos.com/rkt-inspect",
		"HelloDependencies",
	}

	for _, expected := range expectedList {
		if err := expectWithOutput(child, expected); err != nil {
			t.Fatalf("Expected %q but not found: %v", expected, err)
		}
	}

	if err := child.Wait(); err != nil {
		t.Fatalf("rkt didn't terminate correctly: %v", err)
	}

}
