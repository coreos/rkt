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
	"testing"

	"github.com/coreos/rkt/tests/testutils"
)

// Check the version of the Go compiler used to compile rkt
func TestVersion(t *testing.T) {
	ctx := testutils.NewRktRunCtx()
	defer ctx.Cleanup()

	rktCmd := fmt.Sprintf("%s version", ctx.Cmd())
	child := spawnOrFail(t, rktCmd)
	defer child.Wait()

	match := `Go Version: (go[0-9\.]*)`
	result, out, err := expectRegexWithOutput(child, match)
	if err != nil || len(result) != 2 {
		t.Fatalf("could not find Go version. Error: %v\nOutput: %v", err, out)
	}
	goVersion := result[1]

	switch goVersion {
	case "go1.5.1":
		fallthrough
	case "go1.5.2":
		// https://groups.google.com/forum/#!topic/golang-announce/MEATuOi_ei4
		// https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2015-8618
		t.Fatalf("Go Version %s should not be used", goVersion)
	default:
		t.Logf("Go Version: %s", goVersion)
	}
}
