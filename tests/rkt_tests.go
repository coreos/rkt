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
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/steveeJ/gexpect"
)

// dirDesc structure manages one directory and provides an option for
// rkt invocations
type dirDesc struct {
	dir    string // directory path
	desc   string // directory description, mostly for failure cases
	prefix string // temporary directory prefix
	option string // rkt option for given directory
}

// newDirDesc creates dirDesc instance managing a temporary directory.
func newDirDesc(prefix, desc, option string) *dirDesc {
	dir := &dirDesc{
		dir:    "",
		desc:   desc,
		prefix: prefix,
		option: option,
	}
	dir.reset()
	return dir
}

// reset removes the managed directory and recreates it
func (d *dirDesc) reset() {
	d.cleanup()
	dir, err := ioutil.TempDir("", d.prefix)
	if err != nil {
		panic(fmt.Sprintf("Failed to create temporary %s directory: %v", d.desc, err))
	}
	d.dir = dir
}

// cleanup removes the managed directory. After cleanup this instance
// cannot be used for anything, until it is reset.
func (d *dirDesc) cleanup() {
	if d.dir == "" {
		return
	}
	if err := os.RemoveAll(d.dir); err != nil {
		panic(fmt.Sprintf("Failed to remove temporary %s directory %q: %s", d.desc, d.dir, err))
	}
	d.dir = ""
}

// rktOption returns option for rkt invocation
func (d *dirDesc) rktOption() string {
	d.ensureValid()
	return fmt.Sprintf("--%s=%s", d.option, d.dir)
}

func (d *dirDesc) ensureValid() {
	if d.dir == "" {
		panic(fmt.Sprintf("A temporary %s directory is not set up", d.desc))
	}
}

type rktRunCtx struct {
	directories []*dirDesc
	useDefaults bool
	mds         *exec.Cmd
}

func newRktRunCtx() *rktRunCtx {
	return &rktRunCtx{
		directories: []*dirDesc{
			newDirDesc("datadir-", "data", "data"),
			newDirDesc("localdir-", "local configuration", "local-config"),
			newDirDesc("systemdir-", "system configuration", "system-config"),
		},
	}
}

func (ctx *rktRunCtx) launchMDS() error {
	ctx.mds = exec.Command(ctx.rktBin(), "metadata-service")
	return ctx.mds.Start()
}

func (ctx *rktRunCtx) dataDir() string {
	return ctx.dir(0)
}

func (ctx *rktRunCtx) localDir() string {
	return ctx.dir(1)
}

func (ctx *rktRunCtx) systemDir() string {
	return ctx.dir(2)
}

func (ctx *rktRunCtx) dir(idx int) string {
	ctx.ensureValid()
	if idx < len(ctx.directories) {
		return ctx.directories[idx].dir
	}
	panic("Directory index out of bounds")
}

func (ctx *rktRunCtx) reset() {
	ctx.runGC()
	for _, d := range ctx.directories {
		d.reset()
	}
}

func (ctx *rktRunCtx) cleanup() {
	if ctx.mds != nil {
		ctx.mds.Process.Kill()
		ctx.mds.Wait()
		os.Remove("/run/rkt/metadata-svc.sock")
	}

	ctx.runGC()
	for _, d := range ctx.directories {
		d.cleanup()
	}
}

func (ctx *rktRunCtx) runGC() {
	rktArgs := append(ctx.rktOptions(),
		"gc",
		"--grace-period=0s",
	)
	if err := exec.Command(ctx.rktBin(), rktArgs...).Run(); err != nil {
		panic(fmt.Sprintf("Failed to run gc: %v", err))
	}
}

func (ctx *rktRunCtx) cmd() string {
	return fmt.Sprintf("%s %s",
		ctx.rktBin(),
		strings.Join(ctx.rktOptions(), " "),
	)
}

func (ctx *rktRunCtx) rktBin() string {
	rkt := os.Getenv("RKT")
	if rkt == "" {
		panic("Cannot run rkt: RKT env var is not specified")
	}
	abs, err := filepath.Abs(rkt)
	if err != nil {
		abs = rkt
	}
	return abs
}

func (ctx *rktRunCtx) rktOptions() []string {
	ctx.ensureValid()
	opts := make([]string, 0, len(ctx.directories))
	for _, d := range ctx.directories {
		opts = append(opts, d.rktOption())
	}
	return opts
}

func (ctx *rktRunCtx) ensureValid() {
	for _, d := range ctx.directories {
		d.ensureValid()
	}
}

func expectCommon(p *gexpect.ExpectSubprocess, searchString string, timeout time.Duration) error {
	var err error

	p.Capture()
	if timeout == 0 {
		err = p.Expect(searchString)
	} else {
		err = p.ExpectTimeout(searchString, timeout)
	}
	if err != nil {
		return fmt.Errorf(string(p.Collect()))
	}

	return nil
}

func expectWithOutput(p *gexpect.ExpectSubprocess, searchString string) error {
	return expectCommon(p, searchString, 0)
}

func expectRegexWithOutput(p *gexpect.ExpectSubprocess, searchPattern string) ([]string, string, error) {
	return p.ExpectRegexFindWithOutput(searchPattern)
}

func expectRegexTimeoutWithOutput(p *gexpect.ExpectSubprocess, searchPattern string, timeout time.Duration) ([]string, string, error) {
	return p.ExpectTimeoutRegexFindWithOutput(searchPattern, timeout)
}

func expectTimeoutWithOutput(p *gexpect.ExpectSubprocess, searchString string, timeout time.Duration) error {
	return expectCommon(p, searchString, timeout)
}

func patchACI(inputFileName, newFileName string, args ...string) string {
	var allArgs []string

	actool := os.Getenv("ACTOOL")
	if actool == "" {
		panic("Cannot create ACI: ACTOOL env var is not specified")
	}
	tmpDir := os.Getenv("FUNCTIONAL_TMP")
	if tmpDir == "" {
		panic("Cannot create ACI: FUNCTIONAL_TMP env var is not specified")
	}
	imagePath, err := filepath.Abs(filepath.Join(tmpDir, newFileName))
	if err != nil {
		panic(fmt.Sprintf("Cannot create ACI: %v\n", err))
	}
	allArgs = append(allArgs, "patch-manifest")
	allArgs = append(allArgs, "--no-compression")
	allArgs = append(allArgs, "--overwrite")
	allArgs = append(allArgs, args...)
	allArgs = append(allArgs, inputFileName)
	allArgs = append(allArgs, imagePath)

	output, err := exec.Command(actool, allArgs...).CombinedOutput()
	if err != nil {
		panic(fmt.Sprintf("Cannot create ACI: %v: %s\n", err, output))
	}
	return imagePath
}

func patchTestACI(newFileName string, args ...string) string {
	image := os.Getenv("RKT_INSPECT_IMAGE")
	if image == "" {
		panic("Cannot create ACI: RKT_INSPECT_IMAGE env var is not specified")
	}
	return patchACI(image, newFileName, args...)
}

func getHash(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("error opening file: %v", err)
	}

	hash := sha512.New()
	r := io.TeeReader(f, hash)

	if _, err := io.Copy(ioutil.Discard, r); err != nil {
		return "", fmt.Errorf("error reading file: %v", err)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

func importImageAndFetchHash(t *testing.T, ctx *rktRunCtx, img string) string {
	// Import the test image into store manually.
	child, err := gexpect.Spawn(fmt.Sprintf("%s --insecure-skip-verify fetch %s", ctx.cmd(), img))
	if err != nil {
		t.Fatalf("Cannot exec rkt: %v", err)
	}

	// Read out the image hash.
	result, out, err := expectRegexWithOutput(child, "sha512-[0-9a-f]{32}")
	if err != nil || len(result) != 1 {
		t.Fatalf("Error: %v\nOutput: %v", err, out)
	}

	err = child.Wait()
	if err != nil {
		t.Fatalf("rkt didn't terminate correctly: %v", err)
	}

	return result[0]
}

func patchImportAndFetchHash(image string, patches []string, t *testing.T, ctx *rktRunCtx) string {
	imagePath := patchTestACI(image, patches...)
	defer os.Remove(imagePath)

	return importImageAndFetchHash(t, ctx, imagePath)
}

func runGC(t *testing.T, ctx *rktRunCtx) {
	cmd := fmt.Sprintf("%s gc --grace-period=0s", ctx.cmd())
	child, err := gexpect.Spawn(cmd)
	if err != nil {
		t.Fatalf("Cannot exec rkt: %v", err)
	}

	err = child.Wait()
	if err != nil {
		t.Fatalf("rkt didn't terminate correctly: %v", err)
	}
}

func removeFromCas(t *testing.T, ctx *rktRunCtx, hash string) {
	cmd := fmt.Sprintf("%s image rm %s", ctx.cmd(), hash)
	child, err := gexpect.Spawn(cmd)
	if err != nil {
		t.Fatalf("Cannot exec rkt: %v", err)
	}

	err = child.Wait()
	if err != nil {
		t.Fatalf("rkt didn't terminate correctly: %v", err)
	}
}
