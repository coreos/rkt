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
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/ThomasRooney/gexpect"
	taas "github.com/coreos/rkt/tests/test-auth-server/aci"
)

func TestAuthSanity(t *testing.T) {
	ctx := newRktRunCtx()
	defer ctx.cleanup()
	server := runServer(t, taas.None)
	defer server.Close()
	expectedRunRkt(ctx, t, server.URL, "sanity", authSuccessfulDownload)
}

const (
	authSuccessfulDownload = "Authentication succeeded."
	authFailedDownload     = "error downloading ACI: bad HTTP status code: 401"
)

type authConfDir int

const (
	authConfDirNone authConfDir = iota
	authConfDirLocal
	authConfDirSystem
)

type genericAuthTest struct {
	name         string
	confDir      authConfDir
	expectedLine string
}

func TestAuthBasic(t *testing.T) {
	tests := []genericAuthTest{
		{"basic-no-config", authConfDirNone, authFailedDownload},
		{"basic-local-config", authConfDirLocal, authSuccessfulDownload},
		{"basic-system-config", authConfDirSystem, authSuccessfulDownload},
	}
	testAuthGeneric(t, taas.Basic, tests)
}

func TestAuthOauth(t *testing.T) {
	tests := []genericAuthTest{
		{"oauth-no-config", authConfDirNone, authFailedDownload},
		{"oauth-local-config", authConfDirLocal, authSuccessfulDownload},
		{"oauth-system-config", authConfDirSystem, authSuccessfulDownload},
	}
	testAuthGeneric(t, taas.Oauth, tests)
}

func testAuthGeneric(t *testing.T, auth taas.Type, tests []genericAuthTest) {
	server := runServer(t, auth)
	defer server.Close()
	ctx := newRktRunCtx()
	defer ctx.cleanup()
	for _, tt := range tests {
		switch tt.confDir {
		case authConfDirNone:
			// no config to write
		case authConfDirLocal:
			writeConfig(t, ctx.localDir(), "test.json", server.Conf)
		case authConfDirSystem:
			writeConfig(t, ctx.systemDir(), "test.json", server.Conf)
		default:
			panic("Wrong config directory")
		}
		expectedRunRkt(ctx, t, server.URL, tt.name, tt.expectedLine)
		ctx.reset()
	}
}

func TestAuthOverride(t *testing.T) {
	ctx := newRktRunCtx()
	defer ctx.cleanup()
	server := runServer(t, taas.Oauth)
	defer server.Close()
	tests := []struct {
		systemConfig         string
		localConfig          string
		name                 string
		resultBeforeOverride string
		resultAfterOverride  string
	}{
		{server.Conf, getInvalidOAuthConfig(server.Conf), "valid-system-invalid-local", authSuccessfulDownload, authFailedDownload},
		{getInvalidOAuthConfig(server.Conf), server.Conf, "invalid-system-valid-local", authFailedDownload, authSuccessfulDownload},
	}
	for _, tt := range tests {
		writeConfig(t, ctx.systemDir(), "test.json", tt.systemConfig)
		expectedRunRkt(ctx, t, server.URL, tt.name+"-1", tt.resultBeforeOverride)
		writeConfig(t, ctx.localDir(), "test.json", tt.localConfig)
		expectedRunRkt(ctx, t, server.URL, tt.name+"-2", tt.resultAfterOverride)
		ctx.reset()
	}
}

func TestAuthIgnore(t *testing.T) {
	server := runServer(t, taas.Oauth)
	defer server.Close()
	testAuthIgnoreBogusFiles(t, server)
	testAuthIgnoreSubdirectories(t, server)
}

func testAuthIgnoreBogusFiles(t *testing.T, server *taas.Server) {
	ctx := newRktRunCtx()
	defer ctx.cleanup()
	writeConfig(t, ctx.systemDir(), "README", "This is system config")
	writeConfig(t, ctx.localDir(), "README", "This is local config")
	writeConfig(t, ctx.systemDir(), "test.notjson", server.Conf)
	writeConfig(t, ctx.localDir(), "test.notjson", server.Conf)
	expectedRunRkt(ctx, t, server.URL, "oauth-bogus-files", authFailedDownload)
}

func testAuthIgnoreSubdirectories(t *testing.T, server *taas.Server) {
	ctx := newRktRunCtx()
	defer ctx.cleanup()
	localSubdir := filepath.Join(ctx.localDir(), "subdir")
	systemSubdir := filepath.Join(ctx.systemDir(), "subdir")
	writeConfig(t, localSubdir, "test.json", server.Conf)
	writeConfig(t, systemSubdir, "test.json", server.Conf)
	expectedRunRkt(ctx, t, server.URL, "oauth-subdirectories", authFailedDownload)
}

func runServer(t *testing.T, auth taas.Type) *taas.Server {
	actool := os.Getenv("ACTOOL")
	if actool == "" {
		panic("Cannot run test ACI server: ACTOOL env var is not specified")
	}
	gotool := os.Getenv("GO")
	if gotool == "" {
		panic("Cannot run test ACI server: GO env var is not specified")
	}
	server, err := taas.NewServerWithPaths(auth, 20, actool, gotool)
	if err != nil {
		t.Fatalf("Could not start server: %v", err)
	}
	go serverHandler(t, server)
	return server
}

func serverHandler(t *testing.T, server *taas.Server) {
	for {
		select {
		case msg, ok := <-server.Msg:
			if ok {
				t.Logf("server: %v", msg)
			}
		}
	}
}

// expectedRunRkt tries to fetch and run a prog.aci from host within
// given directory on host. Note that directory can be anything - it's
// useful for ensuring that image name is unique and for descriptive
// purposes.
func expectedRunRkt(ctx *rktRunCtx, t *testing.T, host, dir, line string) {
	cmd := fmt.Sprintf(`%s --debug --insecure-skip-verify run %s/%s/prog.aci`, ctx.cmd(), host, dir)
	t.Logf("Running rkt: %s", cmd)
	child, err := gexpect.Spawn(cmd)
	if err != nil {
		t.Fatalf("Failed to run rkt: %v", err)
	}
	defer child.Wait()
	if err := expectWithOutput(child, line); err != nil {
		t.Fatalf("Didn't receive expected output %q: %v", line, err)
	}
}

func writeConfig(t *testing.T, baseDir, filename, contents string) {
	dir := authDir(baseDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("Failed to create config directory %q: %v", dir, err)
	}
	path := filepath.Join(dir, filename)
	os.Remove(path)
	if err := ioutil.WriteFile(path, []byte(contents), 0644); err != nil {
		t.Fatalf("Failed to write file %q: %v", path, err)
	}
}

func authDir(confDir string) string {
	return filepath.Join(confDir, "auth.d")
}

func getInvalidOAuthConfig(conf string) string {
	return strings.Replace(conf, "sometoken", "someobviouslywrongtoken", 1)
}
