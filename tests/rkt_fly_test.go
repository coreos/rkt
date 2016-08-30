// Copyright 2016 The rkt Authors
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

// +build fly

package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/appc/spec/schema"
	"github.com/appc/spec/schema/types"
	"github.com/coreos/rkt/tests/testutils"
)

func TestFlyNetns(t *testing.T) {
	testImageArgs := []string{"--exec=/inspect --print-netns"}
	testImage := patchTestACI("rkt-inspect-stage1-fly.aci", testImageArgs...)
	defer os.Remove(testImage)

	ctx := testutils.NewRktRunCtx()
	defer ctx.Cleanup()

	cmd := fmt.Sprintf("%s --debug --insecure-options=image run %s", ctx.Cmd(), testImage)
	child := spawnOrFail(t, cmd)
	ctx.RegisterChild(child)
	defer waitOrFail(t, child, 0)

	expectedRegex := `NetNS: (net:\[\d+\])`
	result, out, err := expectRegexWithOutput(child, expectedRegex)
	if err != nil {
		t.Fatalf("Error: %v\nOutput: %v", err, out)
	}

	ns, err := os.Readlink("/proc/self/ns/net")
	if err != nil {
		t.Fatalf("Cannot evaluate NetNS symlink: %v", err)
	}

	if nsChanged := ns != result[1]; nsChanged {
		t.Fatalf("container left host netns")
	}
}

func TestFlyMountCLI(t *testing.T) {
	tmpDir := createTempDirOrPanic("rkt-mount-test-")
	defer os.RemoveAll(tmpDir)
	mountSrcFile := filepath.Join(tmpDir, "hello")
	if err := ioutil.WriteFile(mountSrcFile, []byte("world"), 0600); err != nil {
		t.Fatalf("Cannot write file: %v", err)
	}

	testImageArgs := []string{fmt.Sprintf("--exec=/inspect --read-file --file-name %s", mountSrcFile)}
	testImage := patchTestACI("rkt-inspect-stage1-fly.aci", testImageArgs...)
	defer os.Remove(testImage)

	ctx := testutils.NewRktRunCtx()
	defer ctx.Cleanup()

	testParams := []struct {
		mountParam   string
		expectedExit int
	}{
		{
			fmt.Sprintf(
				"--volume=test,kind=host,source=%s --mount volume=test,target=%s",
				mountSrcFile, mountSrcFile,
			),
			0,
		},
		{
			fmt.Sprintf(
				"--volume=test1,kind=host,source=%s --mount volume=test1,target=%s --volume=test2,kind=host,source=%s --mount volume=test1,target=%s",
				mountSrcFile, mountSrcFile,
				mountSrcFile, mountSrcFile,
			),
			1, /* TODO: decide on consistency with other stage1s */
		},
	}

	for _, testParam := range testParams {
		cmd := fmt.Sprintf("%s --debug --insecure-options=image run %s %s", ctx.Cmd(), testImage, testParam.mountParam)
		child := spawnOrFail(t, cmd)
		ctx.RegisterChild(child)
		waitOrFail(t, child, testParam.expectedExit)
	}
}

// TODO: unite this with rkt_run_pod_manifest_test.go
type imagePatch struct {
	name    string
	patches []string
}

const baseAppName = "rkt-inspect"

func verifyHostFile(t *testing.T, tmpdir, filename string, i int, expectedResult string) {
	filePath := path.Join(tmpdir, filename)
	defer os.Remove(filePath)

	// Verify the file is written to host.
	if strings.Contains(expectedResult, "host:") {
		data, err := ioutil.ReadFile(filePath)
		if err != nil {
			t.Fatalf("%d: Cannot read the host file: %v", i, err)
		}
		if string(data) != expectedResult {
			t.Fatalf("%d: Expecting %q in the host file, but saw %q", i, expectedResult, data)
		}
	}
}

func TestFlyMountPodManifest(t *testing.T) {
	ctx := testutils.NewRktRunCtx()
	defer ctx.Cleanup()

	tmpdir := createTempDirOrPanic("rkt-tests.")
	defer os.RemoveAll(tmpdir)

	tests := []struct {
		// [image name]:[image patches]
		images         []imagePatch
		podManifest    *schema.PodManifest
		expectedExit   int
		expectedResult string
	}{
		{
			// Simple read after write with volume mounted in a read-only rootfs.
			[]imagePatch{
				{"rkt-test-run-pod-manifest-read-only-rootfs-vol-rw.aci", []string{}},
			},
			&schema.PodManifest{
				Apps: []schema.RuntimeApp{
					{
						Name: baseAppName,
						App: &types.App{
							Exec:  []string{"/inspect", "--write-file", "--read-file"},
							User:  "0",
							Group: "0",
							Environment: []types.EnvironmentVariable{
								{"FILE", "/dir1/file"},
								{"CONTENT", "host:foo"},
							},
							MountPoints: []types.MountPoint{
								{
									Name:     "dir1",
									Path:     "/dir1",
									ReadOnly: false,
								},
							},
						},
						ReadOnlyRootFS: true,
					},
				},
				Volumes: []types.Volume{
					{
						Name:   "dir1",
						Kind:   "host",
						Source: tmpdir,
					},
				},
			},
			0,
			"host:foo",
		},
	}

	for i, tt := range tests {
		var hashesToRemove []string
		for j, v := range tt.images {
			hash, err := patchImportAndFetchHash(v.name, v.patches, t, ctx)
			if err != nil {
				t.Fatalf("%v", err)
			}
			hashesToRemove = append(hashesToRemove, hash)
			imgName := types.MustACIdentifier(v.name)
			imgID, err := types.NewHash(hash)
			if err != nil {
				t.Fatalf("Cannot generate types.Hash from %v: %v", hash, err)
			}

			ra := &tt.podManifest.Apps[j]
			ra.Image.Name = imgName
			ra.Image.ID = *imgID
		}

		tt.podManifest.ACKind = schema.PodManifestKind
		tt.podManifest.ACVersion = schema.AppContainerVersion

		manifestFile := generatePodManifestFile(t, tt.podManifest)
		defer os.Remove(manifestFile)

		// 1. Test 'rkt run'.
		runCmd := fmt.Sprintf("%s run --mds-register=false --pod-manifest=%s", ctx.Cmd(), manifestFile)
		t.Logf("Running 'run' test #%v", i)
		child := spawnOrFail(t, runCmd)
		ctx.RegisterChild(child)

		if tt.expectedResult != "" {
			if _, out, err := expectRegexWithOutput(child, tt.expectedResult); err != nil {
				t.Errorf("Expected %q but not found: %v\n%s", tt.expectedResult, err, out)
				continue
			}
		}
		waitOrFail(t, child, tt.expectedExit)
		verifyHostFile(t, tmpdir, "file", i, tt.expectedResult)

		// 2. Test 'rkt prepare' + 'rkt run-prepared'.
		rktCmd := fmt.Sprintf("%s --insecure-options=image prepare --pod-manifest=%s",
			ctx.Cmd(), manifestFile)
		uuid := runRktAndGetUUID(t, rktCmd)

		runPreparedCmd := fmt.Sprintf("%s run-prepared --mds-register=false %s", ctx.Cmd(), uuid)
		t.Logf("Running 'run-prepared' test #%v", i)
		child = spawnOrFail(t, runPreparedCmd)

		if tt.expectedResult != "" {
			if _, out, err := expectRegexWithOutput(child, tt.expectedResult); err != nil {
				t.Errorf("Expected %q but not found: %v\n%s", tt.expectedResult, err, out)
				continue
			}
		}

		waitOrFail(t, child, tt.expectedExit)
		verifyHostFile(t, tmpdir, "file", i, tt.expectedResult)

		// we run the garbage collector and remove the imported images to save
		// space
		runGC(t, ctx)
		for _, h := range hashesToRemove {
			removeFromCas(t, ctx, h)
		}
	}
}
