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
)

func TestRunOverrideExec(t *testing.T) {
	patchTestACI("rkt-exec-override.aci", "--exec=/inspect")
	defer os.Remove("rkt-exec-override.aci")
	ctx := newRktRunCtx()
	defer ctx.cleanup()

	for i, tt := range []struct {
		rktCmd       string
		expectedLine string
	}{
		{
			// Sanity check - make sure no --exec override prints the expected exec invocation
			rktCmd:       fmt.Sprintf("%s --insecure-skip-verify run rkt-exec-override.aci -- --print-exec", ctx.cmd()),
			expectedLine: "inspect execed as: /inspect",
		},
		{
			// Now test overridingt the entrypoint (which is a symlink to /inspect so should behave identically)
			rktCmd:       fmt.Sprintf("%s --insecure-skip-verify run rkt-exec-override.aci --exec /inspect-link -- --print-exec", ctx.cmd()),
			expectedLine: "inspect execed as: /inspect-link",
		},
	} {
		child, err := gexpect.Spawn(tt.rktCmd)
		if err != nil {
			t.Fatalf("%d: cannot exec rkt: %v", i, err)
		}

		if err = expectWithOutput(child, tt.expectedLine); err != nil {
			t.Fatalf("%d: didn't receive expected output %q: %v", i, tt.expectedLine, err)
		}

		if err = child.Wait(); err != nil {
			t.Fatalf("%d: rkt didn't terminate correctly: %v", i, err)
		}

	}

}
