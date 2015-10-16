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

// TestFetchFromFile tests that 'rkt fetch/run/prepare' for a file will always
// fetch the file regardless of the specified behavior (default, store only,
// remote only).
func TestFetchFromFile(t *testing.T) {
	image := "rkt-inspect-implicit-fetch.aci"
	imagePath := patchTestACI(image, "--exec=/inspect")

	defer os.Remove(imagePath)

	tests := []struct {
		args  string
		image string
	}{
		{"--insecure-skip-verify fetch", imagePath},
		{"--insecure-skip-verify fetch --store-only", imagePath},
		{"--insecure-skip-verify fetch --no-store", imagePath},
		{"--insecure-skip-verify run --mds-register=false", imagePath},
		{"--insecure-skip-verify run --mds-register=false --store-only", imagePath},
		{"--insecure-skip-verify run --mds-register=false --no-store", imagePath},
		{"--insecure-skip-verify prepare", imagePath},
		{"--insecure-skip-verify prepare --store-only", imagePath},
		{"--insecure-skip-verify prepare --no-store", imagePath},
	}

	for _, tt := range tests {
		testFetchFromFile(t, tt.args, tt.image)
	}
}

func testFetchFromFile(t *testing.T, arg string, image string) {
	fetchFromFileMsg := fmt.Sprintf("using image from file %s", image)

	ctx := newRktRunCtx()
	defer ctx.cleanup()

	cmd := fmt.Sprintf("%s %s %s", ctx.cmd(), arg, image)

	// 1. Run cmd, should get $fetchFromFileMsg.
	child := spawnOrFail(t, cmd)
	if err := expectWithOutput(child, fetchFromFileMsg); err != nil {
		t.Fatalf("%q should be found: %v", fetchFromFileMsg, err)
	}
	child.Wait()

	// 1. Run cmd again, should get $fetchFromFileMsg.
	runRktAndCheckOutput(t, cmd, fetchFromFileMsg, false)
}

// TestFetch tests that 'rkt fetch/run/prepare' for any type (image name string
// or URL) except file:// URL will work with the default, store only
// (--store-only) and remote only (--no-store) behaviors.
func TestFetch(t *testing.T) {
	image := "rkt-inspect-implicit-fetch.aci"
	imagePath := patchTestACI(image, "--exec=/inspect")

	defer os.Remove(imagePath)

	tests := []struct {
		args     string
		image    string
		finalURL string
	}{
		{"--insecure-skip-verify fetch", "coreos.com/etcd:v2.1.2", "https://github.com/coreos/etcd/releases/download/v2.1.2/etcd-v2.1.2-linux-amd64.aci"},
		{"--insecure-skip-verify fetch", "https://github.com/coreos/etcd/releases/download/v2.1.2/etcd-v2.1.2-linux-amd64.aci", ""},
		{"--insecure-skip-verify fetch", "docker://busybox", ""},
		{"--insecure-skip-verify fetch", "docker://busybox:latest", ""},
		{"--insecure-skip-verify run --mds-register=false", "coreos.com/etcd:v2.1.2", "https://github.com/coreos/etcd/releases/download/v2.1.2/etcd-v2.1.2-linux-amd64.aci"},
		{"--insecure-skip-verify run --mds-register=false", "https://github.com/coreos/etcd/releases/download/v2.1.2/etcd-v2.1.2-linux-amd64.aci", ""},
		{"--insecure-skip-verify run --mds-register=false", "docker://busybox", ""},
		{"--insecure-skip-verify run --mds-register=false", "docker://busybox:latest", ""},
		{"--insecure-skip-verify prepare", "https://github.com/coreos/etcd/releases/download/v2.1.2/etcd-v2.1.2-linux-amd64.aci", ""},
		{"--insecure-skip-verify prepare", "coreos.com/etcd:v2.1.2", "https://github.com/coreos/etcd/releases/download/v2.1.2/etcd-v2.1.2-linux-amd64.aci"},
		{"--insecure-skip-verify prepare", "docker://busybox", ""},
		{"--insecure-skip-verify prepare", "docker://busybox:latest", ""},
	}

	for _, tt := range tests {
		testFetchDefault(t, tt.args, tt.image, tt.finalURL)
		testFetchStoreOnly(t, tt.args, tt.image, tt.finalURL)
		testFetchNoStore(t, tt.args, tt.image, tt.finalURL)
	}
}

func testFetchDefault(t *testing.T, arg string, image string, finalURL string) {
	remoteFetchMsgTpl := `remote fetching from url %s`
	storeMsgTpl := `using image from local store for .* %s`
	if finalURL == "" {
		finalURL = image
	}
	remoteFetchMsg := fmt.Sprintf(remoteFetchMsgTpl, finalURL)
	storeMsg := fmt.Sprintf(storeMsgTpl, image)

	ctx := newRktRunCtx()
	defer ctx.cleanup()

	cmd := fmt.Sprintf("%s %s %s", ctx.cmd(), arg, image)

	// 1. Run cmd with the image not available in the store, should get $remoteFetchMsg.
	child := spawnOrFail(t, cmd)
	if err := expectWithOutput(child, remoteFetchMsg); err != nil {
		t.Fatalf("%q should be found: %v", remoteFetchMsg, err)
	}
	child.Wait()

	// 2. Run cmd with the image available in the store, should get $storeMsg.
	runRktAndCheckRegexOutput(t, cmd, storeMsg)
}

func testFetchStoreOnly(t *testing.T, args string, image string, finalURL string) {
	cannotFetchMsgTpl := `unable to fetch image for .* %s`
	storeMsgTpl := `using image from local store for .* %s`
	cannotFetchMsg := fmt.Sprintf(cannotFetchMsgTpl, image)
	storeMsg := fmt.Sprintf(storeMsgTpl, image)

	ctx := newRktRunCtx()
	defer ctx.cleanup()

	cmd := fmt.Sprintf("%s --store-only %s %s", ctx.cmd(), args, image)

	// 1. Run cmd with the image not available in the store should get $cannotFetchMsg.
	runRktAndCheckRegexOutput(t, cmd, cannotFetchMsg)

	importImageAndFetchHash(t, ctx, image)

	// 2. Run cmd with the image available in the store, should get $storeMsg.
	runRktAndCheckRegexOutput(t, cmd, storeMsg)
}

func testFetchNoStore(t *testing.T, args string, image string, finalURL string) {
	remoteFetchMsgTpl := `remote fetching from url %s`
	remoteFetchMsg := fmt.Sprintf(remoteFetchMsgTpl, finalURL)

	ctx := newRktRunCtx()
	defer ctx.cleanup()

	importImageAndFetchHash(t, ctx, image)

	cmd := fmt.Sprintf("%s --no-store %s %s", ctx.cmd(), args, image)

	// 1. Run cmd with the image available in the store, should get $remoteFetchMsg.
	child := spawnOrFail(t, cmd)
	if err := expectWithOutput(child, remoteFetchMsg); err != nil {
		t.Fatalf("%q should be found: %v", remoteFetchMsg, err)
	}
	child.Wait()
}
