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
	"strings"
	"testing"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/ThomasRooney/gexpect"
)

func TestSuccess(t *testing.T) {
	patchTestACI("rkt-inspect-exit0.aci", "--exec=/inspect --print-msg=Hello --exit-code=0")
	defer os.Remove("rkt-inspect-exit0.aci")
	ctx := newRktRunCtx()
	defer ctx.cleanup()

	child, err := gexpect.Spawn(fmt.Sprintf("%s --debug --insecure-skip-verify run ./rkt-inspect-exit0.aci", ctx.cmd()))
	if err != nil {
		t.Fatalf("Cannot exec rkt")
	}
	err = child.Expect("Hello")
	if err != nil {
		t.Fatalf("Missing hello: %v", err)
	}
	forbidden := "main process exited, code=exited, status="
	_, receiver := child.AsyncInteractChannels()
	for {
		msg, open := <-receiver
		if !open {
			break
		}
		if strings.Contains(msg, forbidden) {
			t.Fatalf("Forbidden text received")
		}
	}

	err = child.Wait()
	if err != nil {
		t.Fatalf("rkt didn't terminate correctly: %v", err)
	}
}

func TestFailure(t *testing.T) {
	patchTestACI("rkt-inspect-exit20.aci", "--exec=/inspect --print-msg=Hello --exit-code=20")
	defer os.Remove("rkt-inspect-exit20.aci")
	ctx := newRktRunCtx()
	defer ctx.cleanup()

	child, err := gexpect.Spawn(fmt.Sprintf("%s --debug --insecure-skip-verify run ./rkt-inspect-exit20.aci", ctx.cmd()))
	if err != nil {
		t.Fatalf("Cannot exec rkt")
	}
	err = child.Expect("Hello")
	if err != nil {
		t.Fatalf("Missing hello: %v", err)
	}
	err = child.Expect("process exited, code=exited, status=20")
	if err != nil {
		t.Fatalf("Missing exit status: %v", err)
	}

	err = child.Wait()
	if err != nil {
		t.Fatalf("rkt didn't terminate correctly: %v", err)
	}
}
