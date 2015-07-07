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

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/ThomasRooney/gexpect"
	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/syndtr/gocapability/capability"
)

var capsTests = []struct {
	testName            string
	capIsolator         string
	capa                capability.Cap
	capInStage1Expected bool
	capInStage2Expected bool
	nonrootCapExpected  bool
}{
	{
		testName:            "Check we don't have CAP_NET_ADMIN without isolator",
		capIsolator:         "",
		capa:                capability.CAP_NET_ADMIN,
		capInStage1Expected: false,
		capInStage2Expected: false,
		nonrootCapExpected:  false,
	},
	{
		testName:            "Check we have CAP_MKNOD without isolator",
		capIsolator:         "",
		capa:                capability.CAP_MKNOD,
		capInStage1Expected: true,
		capInStage2Expected: true,
		nonrootCapExpected:  true,
	},
	{
		testName:            "Check we have CAP_NET_ADMIN with an isolator",
		capIsolator:         "CAP_NET_ADMIN,CAP_NET_BIND_SERVICE",
		capa:                capability.CAP_NET_ADMIN,
		capInStage1Expected: true,
		capInStage2Expected: true,
		nonrootCapExpected:  true,
	},
	{
		testName:            "Check we have CAP_NET_BIND_SERVICE with an isolator",
		capIsolator:         "CAP_NET_ADMIN,CAP_NET_BIND_SERVICE",
		capa:                capability.CAP_NET_BIND_SERVICE,
		capInStage1Expected: true,
		capInStage2Expected: true,
		nonrootCapExpected:  true,
	},
	{
		testName:            "Check we don't have CAP_NET_ADMIN with an isolator setting CAP_NET_BIND_SERVICE",
		capIsolator:         "CAP_NET_BIND_SERVICE",
		capa:                capability.CAP_NET_ADMIN,
		capInStage1Expected: false,
		capInStage2Expected: false,
		nonrootCapExpected:  false,
	},
}

func TestCaps(t *testing.T) {
	ctx := newRktRunCtx()
	defer ctx.cleanup()

	for i, tt := range capsTests {
		stage1Args := []string{"--exec=/inspect --print-caps-pid=1 --print-user"}
		stage2Args := []string{"--exec=/inspect --print-caps-pid=0 --print-user"}
		if tt.capIsolator != "" {
			stage1Args = append(stage1Args, "--capability="+tt.capIsolator)
			stage2Args = append(stage2Args, "--capability="+tt.capIsolator)
		}
		stage1FileName := patchTestACI("rkt-inspect-print-caps-stage1.aci", stage1Args...)
		stage2FileName := patchTestACI("rkt-inspect-print-caps-stage2.aci", stage2Args...)
		defer os.Remove(stage1FileName)
		defer os.Remove(stage2FileName)
		stageFileNames := []string{stage1FileName, stage2FileName}

		for _, stage := range []int{1, 2} {
			t.Logf("Running test #%v: %v [stage %v]", i, tt.testName, stage)

			cmd := fmt.Sprintf("%s --debug --insecure-skip-verify run --set-env=CAPABILITY=%d %s", ctx.cmd(), int(tt.capa), stageFileNames[stage-1])
			t.Logf("Command: %v", cmd)
			child, err := gexpect.Spawn(cmd)
			if err != nil {
				t.Fatalf("Cannot exec rkt #%v: %v", i, err)
			}

			expectedLine := tt.capa.String()
			if (stage == 1 && tt.capInStage1Expected) || (stage == 2 && tt.capInStage2Expected) {
				expectedLine += "=enabled"
			} else {
				expectedLine += "=disabled"
			}
			err = expectWithOutput(child, expectedLine)
			if err != nil {
				t.Fatalf("Expected %q but not found: %v", expectedLine, err)
			}

			err = expectWithOutput(child, "User: uid=0 euid=0 gid=0 egid=0")
			if err != nil {
				t.Fatalf("Expected user 0 but not found: %v", err)
			}

			err = child.Wait()
			if err != nil {
				t.Fatalf("rkt didn't terminate correctly: %v", err)
			}
		}
		ctx.reset()
	}
}

func TestNonRootCaps(t *testing.T) {
	ctx := newRktRunCtx()
	defer ctx.cleanup()

	for i, tt := range capsTests {
		args := []string{"--exec=/inspect --print-caps-pid=0 --print-user", "--user=9000", "--group=9000"}
		if tt.capIsolator != "" {
			args = append(args, "--capability="+tt.capIsolator)
		}
		fileName := patchTestACI("rkt-inspect-print-caps-nonroot.aci", args...)
		defer os.Remove(fileName)

		t.Logf("Running test #%v: %v [non-root]", i, tt.testName)

		cmd := fmt.Sprintf("%s --debug --insecure-skip-verify run --set-env=CAPABILITY=%d %s", ctx.cmd(), int(tt.capa), fileName)
		t.Logf("Command: %v", cmd)
		child, err := gexpect.Spawn(cmd)
		if err != nil {
			t.Fatalf("Cannot exec rkt #%v: %v", i, err)
		}

		expectedLine := tt.capa.String()
		if tt.nonrootCapExpected {
			expectedLine += "=enabled"
		} else {
			expectedLine += "=disabled"
		}
		err = expectWithOutput(child, expectedLine)
		if err != nil {
			t.Fatalf("Expected %q but not found: %v", expectedLine, err)
		}

		err = expectWithOutput(child, "User: uid=9000 euid=9000 gid=9000 egid=9000")
		if err != nil {
			t.Fatalf("Expected user 9000 but not found: %v", err)
		}

		err = child.Wait()
		if err != nil {
			t.Fatalf("rkt didn't terminate correctly: %v", err)
		}
		ctx.reset()
	}
}
