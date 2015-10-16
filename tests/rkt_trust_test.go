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

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/steveeJ/gexpect"
)

func runImage(t *testing.T, ctx *rktRunCtx, imageFile string, expected string, shouldFail bool) {
	cmd := fmt.Sprintf(`%s --debug run --mds-register=false %s`, ctx.cmd(), imageFile)
	runRktAndCheckOutput(t, cmd, expected, shouldFail)
}

func runRktTrust(t *testing.T, ctx *rktRunCtx, prefix string) {
	var cmd string
	if prefix == "" {
		cmd = fmt.Sprintf(`%s trust --root %s`, ctx.cmd(), "key.gpg")
	} else {
		cmd = fmt.Sprintf(`%s trust --prefix %s %s`, ctx.cmd(), prefix, "key.gpg")
	}

	child := spawnOrFail(t, cmd)
	defer waitOrFail(t, child, true)

	expected := "Are you sure you want to trust this key"
	if err := expectWithOutput(child, expected); err != nil {
		t.Fatalf("Expected but didn't find %q in %v", expected, err)
	}

	if err := child.SendLine("yes"); err != nil {
		t.Fatalf("Cannot confirm rkt trust: %s", err)
	}

	if prefix == "" {
		expected = "Added root key at"
	} else {
		expected = fmt.Sprintf(`Added key for prefix "%s" at`, prefix)
	}
	if err := expectWithOutput(child, expected); err != nil {
		t.Fatalf("Expected but didn't find %q in %v", expected, err)
	}
}

func runSignImage(t *testing.T, ctx *rktRunCtx, imageFile string) {
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Cannot get current working directory: %v", err)
	}

	cmd := fmt.Sprintf("gpg --no-default-keyring --secret-keyring %s/secring.gpg --keyring %s/pubring.gpg --default-key D9DCEF41 --output %s.asc --detach-sig %s",
		dir, dir, imageFile, imageFile)
	t.Logf("%s\n", cmd)
	child, err := gexpect.Spawn(cmd)
	if err != nil {
		t.Fatalf("Cannot exec gpg: %s", err)
	}

	err = child.Wait()
	if err != nil {
		t.Fatalf("gpg terminate as expected: %v", err)
	}
}

func TestTrust(t *testing.T) {
	imageFile := patchTestACI("rkt-inspect-trust1.aci", "--exec=/inspect --print-msg=Hello", "--name=rkt-prefix.com/my-app")
	defer os.Remove(imageFile)

	imageFile2 := patchTestACI("rkt-inspect-trust2.aci", "--exec=/inspect --print-msg=Hello", "--name=rkt-alternative.com/my-app")
	defer os.Remove(imageFile2)

	ctx := newRktRunCtx()
	defer ctx.cleanup()

	t.Logf("Run the non-signed image: it should fail\n")
	runImage(t, ctx, imageFile, "error opening signature file", true)

	t.Logf("Sign the images\n")
	runSignImage(t, ctx, imageFile)
	defer os.Remove(imageFile + ".asc")
	runSignImage(t, ctx, imageFile2)
	defer os.Remove(imageFile2 + ".asc")

	t.Logf("Run the signed image without trusting the key: it should fail\n")
	runImage(t, ctx, imageFile, "openpgp: signature made by unknown entity", true)

	t.Logf("Trust the key with the wrong prefix\n")
	runRktTrust(t, ctx, "wrong-prefix.com/my-app")

	t.Logf("Run a signed image with the key installed in the wrong prefix: it should fail\n")
	runImage(t, ctx, imageFile, "openpgp: signature made by unknown entity", true)

	t.Logf("Trust the key with the correct prefix\n")
	runRktTrust(t, ctx, "rkt-prefix.com/my-app")

	t.Logf("Finally, run successfully the signed image\n")
	runImage(t, ctx, imageFile, "Hello", false)
	runImage(t, ctx, imageFile2, "openpgp: signature made by unknown entity", true)

	t.Logf("Trust the key on unrelated prefixes\n")
	runRktTrust(t, ctx, "foo.com")
	runRktTrust(t, ctx, "example.com/my-app")

	t.Logf("But still only the first image can be executed\n")
	runImage(t, ctx, imageFile, "Hello", false)
	runImage(t, ctx, imageFile2, "openpgp: signature made by unknown entity", true)

	t.Logf("Trust the key for all images (rkt trust --root)\n")
	runRktTrust(t, ctx, "")

	t.Logf("Now both images can be executed\n")
	runImage(t, ctx, imageFile, "Hello", false)
	runImage(t, ctx, imageFile2, "Hello", false)
}
