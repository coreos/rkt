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
)

func TestSupplementaryGIDs(t *testing.T) {
	ctx := newRktRunCtx()
	defer ctx.cleanup()

	printSupplGroups := patchTestACI("rkt-inspect-print-supplementary-groups.aci",
		"--supplementary-groups=400,500,1200",
		"--exec=/inspect --print-groups")
	defer os.Remove(printSupplGroups)

	cmd := fmt.Sprintf("%s --debug --insecure-skip-verify run --mds-register=false %s", ctx.cmd(), printSupplGroups)
	t.Logf("Command: %v", cmd)
	runRktAndCheckOutput(t, cmd, "Groups: 0 400 500 1200", false)
}
