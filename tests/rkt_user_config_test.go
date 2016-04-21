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
	"path/filepath"
	"testing"

	"github.com/coreos/rkt/tests/testutils"
)

// TestDNS is checking how rkt fills /etc/resolv.conf
func TestUserConfigNetPlugin(t *testing.T) {
	sleepImage := patchTestACI("rkt-user-config-sleep.aci", "--exec=/inspect --read-stdin")
	defer os.Remove(sleepImage)
	ctx := testutils.NewRktRunCtx()
	defer ctx.Cleanup()
	stubPlugin := getStubCNIPluginPath()
	text := "testing user config together with custom plugin network in custom plugin directory"
	tmpdir, err := ioutil.TempDir("", "rkt-test-user-config-net-plugin-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)
	testFile := filepath.Join(tmpdir, "testfile")

	pathsCfgDir := filepath.Join(ctx.UserDir(), "stage1-cfg", "paths.d")
	if err := os.MkdirAll(pathsCfgDir, 0775); err != nil {
		t.Fatal(err)
	}
	pathsCfgFile := filepath.Join(pathsCfgDir, "stub.json")
	pathsCfg := fmt.Sprintf(`{"rktKind":"paths","rktVersion":"v1","netPluginDirs":{"paths":[%q]}}`, filepath.Dir(stubPlugin))
	if err := ioutil.WriteFile(pathsCfgFile, []byte(pathsCfg), 0755); err != nil {
		t.Fatal(err)
	}

	netCfgDir := filepath.Join(ctx.UserDir(), "stage1-cfg", "net.d")
	if err := os.MkdirAll(netCfgDir, 0775); err != nil {
		t.Fatal(err)
	}
	netCfgFile := filepath.Join(netCfgDir, "stub.json")
	netCfg := fmt.Sprintf(`{"rktKind":"network","rktVersion":"v1","name":"stub","priority":1,"cniConf":{"type":%q,"text":%q,"path":%q}}`, filepath.Base(stubPlugin), text, testFile)
	if err := ioutil.WriteFile(netCfgFile, []byte(netCfg), 0755); err != nil {
		t.Fatal(err)
	}

	sleepCmd := fmt.Sprintf(`%s --debug --insecure-options=image run --net=stub --interactive %s`, ctx.Cmd(), sleepImage)
	child := spawnOrFail(t, sleepCmd)
	defer waitOrFail(t, child, 0)

	if err := expectWithOutput(child, "Enter text:"); err != nil {
		t.Fatalf("Waited for the prompt but not found: %v", err)
	}
	readRaw, err := ioutil.ReadFile(testFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(readRaw) != text {
		t.Fatalf("expected %q, got %q", text, string(readRaw))
	}

	if err := child.SendLine("Bye"); err != nil {
		t.Fatalf("rkt couldn't write to the container: %v", err)
	}
	if err := expectWithOutput(child, "Received text: Bye"); err != nil {
		t.Fatalf("Expected Bye but not found: %v", err)
	}
}
