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
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/schema/types"
	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/steveeJ/gexpect"
)

const (
	rmImageReferenced = `rkt: image ID %q is referenced by some containers, cannot remove.`
	rmImageOk         = "rkt: successfully removed aci for image ID:"

	unreferencedACI = "rkt-unreferencedACI.aci"
	unreferencedApp = "coreos.com/rkt-unreferenced"
	referencedApp   = "coreos.com/rkt-inspect"

	stage1App = "coreos.com/rkt/stage1"
)

func TestImageRunRm(t *testing.T) {
	imageFile := patchTestACI(unreferencedACI, fmt.Sprintf("--name=%s", unreferencedApp))
	defer os.Remove(imageFile)
	ctx := newRktRunCtx()
	defer ctx.cleanup()

	cmd := fmt.Sprintf("%s --insecure-skip-verify fetch %s", ctx.cmd(), imageFile)
	t.Logf("Fetching %s: %v", imageFile, cmd)
	spawnAndWaitOrFail(t, cmd, true)

	// at this point we know that RKT_INSPECT_IMAGE env var is not empty
	referencedACI := os.Getenv("RKT_INSPECT_IMAGE")
	cmd = fmt.Sprintf("%s --insecure-skip-verify run --mds-register=false %s", ctx.cmd(), referencedACI)
	t.Logf("Running %s: %v", referencedACI, cmd)
	spawnAndWaitOrFail(t, cmd, true)

	t.Logf("Retrieving stage1 image ID")
	stage1ImageID, err := getImageId(ctx, stage1App)
	if err != nil {
		t.Fatalf("rkt didn't terminate correctly: %v", err)
	}

	t.Logf("Retrieving %s image ID", referencedApp)
	referencedImageID, err := getImageId(ctx, referencedApp)
	if err != nil {
		t.Fatalf("rkt didn't terminate correctly: %v", err)
	}

	t.Logf("Retrieving %s image ID", unreferencedApp)
	unreferencedImageID, err := getImageId(ctx, unreferencedApp)
	if err != nil {
		t.Fatalf("rkt didn't terminate correctly: %v", err)
	}

	t.Logf("Removing stage1 image (should work)")
	if err := removeImageId(ctx, stage1ImageID, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Logf("Removing image for app %s (should work)", referencedApp)
	if err := removeImageId(ctx, referencedImageID, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Logf("Removing image for app %s (should work)", unreferencedApp)
	if err := removeImageId(ctx, unreferencedImageID, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestImagePrepareRmRun(t *testing.T) {
	imageFile := patchTestACI(unreferencedACI, fmt.Sprintf("--name=%s", unreferencedApp))
	defer os.Remove(imageFile)
	ctx := newRktRunCtx()
	defer ctx.cleanup()

	cmd := fmt.Sprintf("%s --insecure-skip-verify fetch %s", ctx.cmd(), imageFile)
	t.Logf("Fetching %s: %v", imageFile, cmd)
	spawnAndWaitOrFail(t, cmd, true)

	// at this point we know that RKT_INSPECT_IMAGE env var is not empty
	referencedACI := os.Getenv("RKT_INSPECT_IMAGE")
	cmds := strings.Fields(ctx.cmd())
	prepareCmd := exec.Command(cmds[0], cmds[1:]...)
	prepareCmd.Args = append(prepareCmd.Args, "--insecure-skip-verify", "prepare", referencedACI)
	output, err := prepareCmd.Output()
	if err != nil {
		t.Fatalf("Cannot read the output: %v", err)
	}

	podIDStr := strings.TrimSpace(string(output))
	podID, err := types.NewUUID(podIDStr)
	if err != nil {
		t.Fatalf("%q is not a valid UUID: %v", podIDStr, err)
	}

	t.Logf("Retrieving stage1 imageID")
	stage1ImageID, err := getImageId(ctx, stage1App)
	if err != nil {
		t.Fatalf("rkt didn't terminate correctly: %v", err)
	}

	t.Logf("Retrieving %s image ID", referencedApp)
	referencedImageID, err := getImageId(ctx, referencedApp)
	if err != nil {
		t.Fatalf("rkt didn't terminate correctly: %v", err)
	}

	t.Logf("Retrieving %s image ID", unreferencedApp)
	unreferencedImageID, err := getImageId(ctx, unreferencedApp)
	if err != nil {
		t.Fatalf("rkt didn't terminate correctly: %v", err)
	}

	t.Logf("Removing stage1 image (should work)")
	if err := removeImageId(ctx, stage1ImageID, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Logf("Removing image for app %s (should work)", referencedApp)
	if err := removeImageId(ctx, referencedImageID, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Logf("Removing image for app %s (should work)", unreferencedApp)
	if err := removeImageId(ctx, unreferencedImageID, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cmd = fmt.Sprintf("%s run-prepared --mds-register=false %s", ctx.cmd(), podID.String())
	t.Logf("Running %s: %v", referencedACI, cmd)
	spawnAndWaitOrFail(t, cmd, true)
}

func getImageId(ctx *rktRunCtx, name string) (string, error) {
	cmd := fmt.Sprintf(`/bin/sh -c "%s image list --fields=id,name --no-legend | grep %s | awk '{print $1}'"`, ctx.cmd(), name)
	child, err := gexpect.Spawn(cmd)
	if err != nil {
		return "", fmt.Errorf("Cannot exec rkt: %v", err)
	}
	imageID, err := child.ReadLine()
	imageID = strings.TrimSpace(imageID)
	imageID = string(bytes.Trim([]byte(imageID), "\x00"))
	if err != nil {
		return "", fmt.Errorf("Cannot exec: %v", err)
	}
	if err := child.Wait(); err != nil {
		return "", fmt.Errorf("rkt didn't terminate correctly: %v", err)
	}
	return imageID, nil
}

func removeImageId(ctx *rktRunCtx, imageID string, shouldWork bool) error {
	expect := fmt.Sprintf(rmImageReferenced, imageID)
	if shouldWork {
		expect = rmImageOk
	}
	cmd := fmt.Sprintf("%s image rm %s", ctx.cmd(), imageID)
	child, err := gexpect.Spawn(cmd)
	if err != nil {
		return fmt.Errorf("Cannot exec: %v", err)
	}
	if err := expectWithOutput(child, expect); err != nil {
		return fmt.Errorf("Expected %q but not found: %v", expect, err)
	}
	if err := child.Wait(); err != nil {
		return fmt.Errorf("rkt didn't terminate correctly: %v", err)
	}
	return nil
}
