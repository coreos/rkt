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
	"path/filepath"
	"testing"

	"github.com/coreos/rkt/rkt/config"
	"github.com/coreos/rkt/store"
	"github.com/coreos/rkt/tests/testutils"
)

func TestRktInstallStage1(t *testing.T) {
	stubStage1 := testutils.GetValueFromEnvOrPanic("STUB_STAGE1")
	ctx := testutils.NewRktRunCtx()
	defer ctx.Cleanup()

	if !filepath.IsAbs(stubStage1) {
		abs, err := filepath.Abs(stubStage1)
		if err != nil {
			t.Fatalf("failed to get the absolute path to the stub stage1 image (based on %s): %v", abs, err)
		}
		stubStage1 = abs
	}

	cmd := fmt.Sprintf("%s --insecure-options=image install stage1 %s", ctx.Cmd(), stubStage1)
	spawnAndWaitOrFail(t, cmd, true)
	rawHash := fmt.Sprintf("sha512-%s", getHashOrPanic(stubStage1))
	s, err := store.NewStore(ctx.DataDir())
	if err != nil {
		t.Fatalf("failed to open the store at %q: %v", ctx.DataDir(), err)
	}
	hash, err := s.ResolveKey(rawHash)
	if err != nil {
		t.Fatalf("failed to resolve the key for hash %q: %v", rawHash, err)
	}
	im, err := s.GetImageManifest(hash)
	if err != nil {
		t.Fatalf("failed to get manifest for hash %q: %v", hash, err)
	}

	cfg, err := config.GetConfigFrom(ctx.SystemDir())
	if err != nil {
		t.Fatal(err)
	}
	if im.Name.String() != cfg.Stage1.Name {
		t.Fatalf("expected name %q, got %q", im.Name.String(), cfg.Stage1.Name)
	}
	version, ok := im.GetLabel("version")
	if !ok {
		t.Fatal("no version label in manifest")
	}
	if version != cfg.Stage1.Version {
		t.Fatalf("expected version %q, got %q", version, cfg.Stage1.Version)
	}
	if stubStage1 != cfg.Stage1.Location {
		t.Fatalf("expected location %q, got %q", stubStage1, cfg.Stage1.Location)
	}
}
