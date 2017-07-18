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

// +build host coreos src kvm

package main

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/rkt/rkt/tests/testutils"
)

// TestMachineID test that /etc/machine-id gets created/written with the Pod uuid.
func TestMachineID(t *testing.T) {
	imageFile := patchTestACI("rkt-inspect-machine-id.aci", "--exec=/inspect --read-file")
	defer os.Remove(imageFile)

	ctx := testutils.NewRktRunCtx()
	defer ctx.Cleanup()

	rktCmd := fmt.Sprintf(
		"%s prepare --insecure-options=image %s --environment=FILE=/etc/machine-id",
		ctx.Cmd(), imageFile,
	)
	uuid := runRktAndGetUUID(t, rktCmd)
	rktCmd = fmt.Sprintf("%s run-prepared %s", ctx.Cmd(), uuid)
	expected := strings.Replace(uuid, "-", "", -1)
	runRktAndCheckOutput(t, rktCmd, expected, false)
}

// TestMachineID test that /etc/machine-id gets created/written on a readonly rootfs.
func TestMachineIDReadOnly(t *testing.T) {
	imageFile := patchTestACI("rkt-inspect-machine-id-ro.aci", "--exec=/inspect --read-file")
	defer os.Remove(imageFile)

	ctx := testutils.NewRktRunCtx()
	defer ctx.Cleanup()

	rktCmd := fmt.Sprintf(
		"%s prepare --insecure-options=image %s --readonly-rootfs=true --environment=FILE=/etc/machine-id",
		ctx.Cmd(), imageFile,
	)
	uuid := runRktAndGetUUID(t, rktCmd)
	rktCmd = fmt.Sprintf("%s run-prepared %s", ctx.Cmd(), uuid)
	expected := strings.Replace(uuid, "-", "", -1)
	runRktAndCheckOutput(t, rktCmd, expected, false)
}
