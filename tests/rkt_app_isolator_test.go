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
	"strconv"
	"testing"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/ThomasRooney/gexpect"
	"github.com/coreos/rkt/common/cgroup"
)

const (
	// if you change this you need to change tests/image/manifest accordingly
	maxMemoryUsage = 25 * 1024 * 1024 // 25MB
	CPUQuota       = 800              // milli-cores
)

var memoryTest = struct {
	testName     string
	aciBuildArgs []string
	rktArgs      string
}{
	`Check memory isolator`,
	[]string{"--exec=/inspect --print-memorylimit"},
	`--insecure-skip-verify run rkt-inspect-isolators.aci`,
}

var cpuTest = struct {
	testName     string
	aciBuildArgs []string
	rktArgs      string
}{
	`Check CPU quota`,
	[]string{"--exec=/inspect --print-cpuquota"},
	`--insecure-skip-verify run rkt-inspect-isolators.aci`,
}

func TestAppIsolatorMemory(t *testing.T) {
	ok := cgroup.IsIsolatorSupported("memory")
	if !ok {
		t.Skip("Memory isolator not supported.")
	}

	ctx := newRktRunCtx()
	defer ctx.cleanup()

	t.Logf("Running test: %v", memoryTest.testName)

	aciFileName := patchTestACI("rkt-inspect-isolators.aci", memoryTest.aciBuildArgs...)
	defer os.Remove(aciFileName)

	rktCmd := fmt.Sprintf("%s %s", ctx.cmd(), memoryTest.rktArgs)
	t.Logf("Command: %v", rktCmd)
	child, err := gexpect.Spawn(rktCmd)
	if err != nil {
		t.Fatalf("Cannot exec rkt: %v", err)
	}
	expectedLine := "Memory Limit: " + strconv.Itoa(maxMemoryUsage)
	if err := expectWithOutput(child, expectedLine); err != nil {
		t.Fatalf("Didn't receive expected output %q: %v", expectedLine, err)
	}

	err = child.Wait()
	if err != nil {
		t.Fatalf("rkt didn't terminate correctly: %v", err)
	}
}

func TestAppIsolatorCPU(t *testing.T) {
	ok := cgroup.IsIsolatorSupported("cpu")
	if !ok {
		t.Skip("CPU isolator not supported.")
	}

	ctx := newRktRunCtx()
	defer ctx.cleanup()

	t.Logf("Running test: %v", cpuTest.testName)

	aciFileName := patchTestACI("rkt-inspect-isolators.aci", cpuTest.aciBuildArgs...)
	defer os.Remove(aciFileName)

	rktCmd := fmt.Sprintf("%s %s", ctx.cmd(), cpuTest.rktArgs)
	t.Logf("Command: %v", rktCmd)
	child, err := gexpect.Spawn(rktCmd)
	if err != nil {
		t.Fatalf("Cannot exec rkt: %v", err)
	}
	expectedLine := "CPU Quota: " + strconv.Itoa(CPUQuota)
	if err := expectWithOutput(child, expectedLine); err != nil {
		t.Fatalf("Didn't receive expected output %q: %v", expectedLine, err)
	}

	err = child.Wait()
	if err != nil {
		t.Fatalf("rkt didn't terminate correctly: %v", err)
	}

}
