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

package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/coreos/rkt/pkg/selinux"
	"github.com/coreos/rkt/tests/testutils"
)

func TestSelinuxMount(t *testing.T) {
	osInfo := getOSReleaseOrFail(t)
	if osInfo["ID"] != "fedora" || osInfo["VERSION_ID"] != "25" {
		t.Skip("This test is only supported on fedora, and require version >= 25")
	}

	if !selinux.SelinuxEnabled() {
		t.Skip("SELinux is not enabled")
	}

	result, err := exec.Command("getenforce").CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to run 'getenforce': %v", err)
	}
	if strings.TrimSpace(string(result)) != "Enforcing" {
		t.Skip("SELinux is not enforced")
	}

	ctx := testutils.NewRktRunCtx()
	defer ctx.Cleanup()

	tmpFile, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatalf("Cannot create temp file: %v", err)
	}

	name := tmpFile.Name()
	defer os.Remove(name)

	// Write the magic number in the file.
	if _, err := tmpFile.Write([]byte("<<<42>>>")); err != nil {
		t.Fatalf("Cannot write to temp file %q: %v", name, err)
	}
	tmpFile.Close()

	if err := selinux.Setfilecon(name, "system_u:object_r:svirt_sandbox_file_t:s0:c1"); err != nil {
		t.Fatalf("Cannot set selinux context of the file %q: %v", name, err)
	}

	imageFile := patchTestACI("rkt-selinux-test.aci", "--exec=/inspect --read-file")
	defer os.Remove(imageFile)

	tests := []struct {
		selinuxOptions string
		expectedResult string
		expectedError  bool
	}{
		{
			"level:s0:c1",
			"<<<42>>>",
			false,
		},
		{
			"level:s0:c2",
			"",
			true,
		},
	}

	for _, tt := range tests {
		rktCmd := fmt.Sprintf("%s --insecure-options=image run --no-overlay=true --volume=tmpfile,kind=host,source=%s --mount=volume=tmpfile,target=/tmp/tmpfile --selinux-options=%s --set-env=FILE=/tmp/tmpfile %s", ctx.Cmd(), name, tt.selinuxOptions, imageFile)
		t.Logf("%s\n", rktCmd)
		runRktAndCheckOutput(t, rktCmd, tt.expectedResult, tt.expectedError)
	}
}
