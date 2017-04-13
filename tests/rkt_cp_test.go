// Copyright 2017 The rkt Authors
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

// +build host coreos src kvm

package main

import (
	"fmt"
	"os"
	"testing"

	"github.com/rkt/rkt/tests/testutils"
)

// TestRktCp tests 'rkt cp'.
// It copies a file from within a contain and checks whether the file is present.
func TestRktCp(t *testing.T) {
	tmpDir := mustTempDir("rkt-TestRktCp-")
	defer os.RemoveAll(tmpDir)

	ctx := testutils.NewRktRunCtx()
	defer ctx.Cleanup()

	image := patchTestACI("rkt-inspect-sleep.aci", "--exec=/inspect --read-stdin")
	defer os.Remove(image)

	t.Logf("Starting 'sleep' container")
	runCmd := fmt.Sprintf("%s --insecure-options=image run --mds-register=false --interactive %s", ctx.Cmd(), image)
	child := spawnOrFail(t, runCmd)

	if err := expectWithOutput(child, "Enter text:"); err != nil {
		t.Fatalf("Waited for the prompt but not found : %v", err)
	}

	cpCmd := fmt.Sprintf(`/bin/sh -c "%s cp $(%s list | grep running | awk '{print $1}'):/inspect %s"`, ctx.Cmd(), ctx.Cmd(), tmpDir)
	t.Logf("Executing command '%s'", cpCmd)

	//if err := runRktAndCheckRegexOutput(t, cpCmd, ""); err != nil {
	//	t.Errorf("unexpected error: %v", err)
	//}

	output, exitCode := runRkt(t, cpCmd, 0, 0)
	t.Logf("Exit code = %d", exitCode)
	t.Logf("Output = %s", output)

	if err := runRktAndCheckRegexOutput(t, cpCmd, ""); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if _, err := os.Stat(fmt.Sprintf("%s/inspect", tmpDir)); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if err := child.SendLine("Bye"); err != nil {
		t.Fatalf("rkt couldn't write to the container: %v", err)
	}
	if err := expectWithOutput(child, "Received text: Bye"); err != nil {
		t.Fatalf("Expected Bye but not found: %v", err)
	}

	waitOrFail(t, child, 0)
}
