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
	"os"
	"testing"

	"github.com/coreos/rkt/tests/testutils"
)

func TestRktList(t *testing.T) {
	const imgName = "rkt-list-test"

	image := patchTestACI(fmt.Sprintf("%s.aci", imgName), fmt.Sprintf("--name=%s", imgName))
	defer os.Remove(image)

	imageHash := getHashOrPanic(image)
	imgID := ImageId{image, imageHash}

	ctx := testutils.NewRktRunCtx()
	defer ctx.Cleanup()

	// Prepare image
	cmd := fmt.Sprintf("%s --insecure-options=image prepare %s", ctx.Cmd(), imgID.path)
	podUuid := runRktAndGetUUID(t, cmd)

	// Get hash
	imageID := fmt.Sprintf("sha512-%s", imgID.hash[:12])

	tmpDir := createTempDirOrPanic(imgName)
	defer os.RemoveAll(tmpDir)

	// Define tests
	tests := []struct {
		cmd           string
		shouldSucceed bool
		expect        string
	}{
		// Test that pod UUID is in output
		{
			"list --full",
			true,
			podUuid,
		},
		// Test that image name is in output
		{
			"list",
			true,
			imgName,
		},
		// Test that imageID is in output
		{
			"list --full",
			true,
			imageID,
		},
		// Remove the image
		{
			fmt.Sprintf("image rm %s", imageID),
			true,
			"successfully removed",
		},
		// Name should still show up in rkt list
		{
			"list",
			true,
			imgName,
		},
		// Test that imageID is still in output
		{
			"list --full",
			true,
			imageID,
		},
	}

	// Run tests
	for i, tt := range tests {
		runCmd := fmt.Sprintf("%s %s", ctx.Cmd(), tt.cmd)
		t.Logf("Running test #%d, %s", i, runCmd)
		runRktAndCheckOutput(t, runCmd, tt.expect, !tt.shouldSucceed)
	}
}
