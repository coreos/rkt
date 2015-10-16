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
	"syscall"
	"testing"
	"time"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/schema/types"
	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/steveeJ/gexpect"
)

const (
	nobodyUid = uint32(65534)
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
			newDirDesc("datadir-", "data", "dir"),
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

// TODO(jonboulle): clean this up
func (ctx *rktRunCtx) cmdNoConfig() string {
	return fmt.Sprintf("%s %s",
		ctx.rktBin(),
		ctx.directories[0].rktOption(),
	)
}

func (ctx *rktRunCtx) rktBin() string {
	rkt := getValueFromEnvOrPanic("RKT")
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

	actool := getValueFromEnvOrPanic("ACTOOL")
	tmpDir := getValueFromEnvOrPanic("FUNCTIONAL_TMP")

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
	image := getInspectImagePath()
	return patchACI(image, newFileName, args...)
}

func spawnOrFail(t *testing.T, cmd string) *gexpect.ExpectSubprocess {
	t.Logf("Command: %v", cmd)
	child, err := gexpect.Spawn(cmd)
	if err != nil {
		t.Fatalf("Cannot exec rkt: %v", err)
	}
	return child
}

func waitOrFail(t *testing.T, child *gexpect.ExpectSubprocess, shouldSucceed bool) {
	err := child.Wait()
	switch {
	case !shouldSucceed && err == nil:
		t.Fatalf("Expected test to fail but it didn't")
	case shouldSucceed && err != nil:
		t.Fatalf("rkt didn't terminate correctly: %v", err)
	case err != nil && err.Error() != "exit status 1":
		t.Fatalf("rkt terminated with unexpected error: %v", err)
	}
}

func spawnAndWaitOrFail(t *testing.T, cmd string, shouldSucceed bool) {
	child := spawnOrFail(t, cmd)
	waitOrFail(t, child, shouldSucceed)
}

func getValueFromEnvOrPanic(envVar string) string {
	path := os.Getenv(envVar)
	if path == "" {
		panic(fmt.Sprintf("Empty %v environment variable\n", envVar))
	}
	return path
}

func getEmptyImagePath() string {
	return getValueFromEnvOrPanic("RKT_EMPTY_IMAGE")
}

func getInspectImagePath() string {
	return getValueFromEnvOrPanic("RKT_INSPECT_IMAGE")
}

func getHashOrPanic(path string) string {
	hash, err := getHash(path)
	if err != nil {
		panic(fmt.Sprintf("Cannot get hash from file located at %v", path))
	}
	return hash
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

func createTempDirOrPanic(dirName string) string {
	tmpDir, err := ioutil.TempDir("", dirName)
	if err != nil {
		panic(fmt.Sprintf("Cannot create temp dir: %v", err))
	}
	return tmpDir
}

func importImageAndFetchHashAsGid(t *testing.T, ctx *rktRunCtx, img string, gid int) string {
	// Import the test image into store manually.
	cmd := fmt.Sprintf("%s --insecure-skip-verify fetch %s", ctx.cmd(), img)

	// TODO(jonboulle): non-root user breaks trying to read root-written
	// config directories. Should be a better way to approach this. Should
	// config directories be readable by the rkt group too?
	if gid != 0 {
		cmd = fmt.Sprintf("%s --insecure-skip-verify fetch %s", ctx.cmdNoConfig(), img)
	}
	child, err := gexpect.Command(cmd)
	if err != nil {
		t.Fatalf("cannot create rkt command: %v", err)
	}
	if gid != 0 {
		child.Cmd.SysProcAttr = &syscall.SysProcAttr{}
		child.Cmd.SysProcAttr.Credential = &syscall.Credential{Uid: nobodyUid, Gid: uint32(gid)}
	}

	err = child.Start()
	if err != nil {
		t.Fatalf("cannot exec rkt: %v", err)
	}

	// Read out the image hash.
	result, out, err := expectRegexWithOutput(child, "sha512-[0-9a-f]{32}")
	if err != nil || len(result) != 1 {
		t.Fatalf("Error: %v\nOutput: %v", err, out)
	}

	waitOrFail(t, child, true)

	return result[0]
}

func importImageAndFetchHash(t *testing.T, ctx *rktRunCtx, img string) string {
	return importImageAndFetchHashAsGid(t, ctx, img, 0)
}

func patchImportAndFetchHash(image string, patches []string, t *testing.T, ctx *rktRunCtx) string {
	imagePath := patchTestACI(image, patches...)
	defer os.Remove(imagePath)

	return importImageAndFetchHash(t, ctx, imagePath)
}

func runGC(t *testing.T, ctx *rktRunCtx) {
	cmd := fmt.Sprintf("%s gc --grace-period=0s", ctx.cmd())
	spawnAndWaitOrFail(t, cmd, true)
}

func runImageGC(t *testing.T, ctx *rktRunCtx) {
	cmd := fmt.Sprintf("%s image gc", ctx.cmd())
	spawnAndWaitOrFail(t, cmd, true)
}

func removeFromCas(t *testing.T, ctx *rktRunCtx, hash string) {
	cmd := fmt.Sprintf("%s image rm %s", ctx.cmd(), hash)
	spawnAndWaitOrFail(t, cmd, true)
}

func runRktAndGetUUID(t *testing.T, rktCmd string) string {
	child := spawnOrFail(t, rktCmd)
	defer waitOrFail(t, child, true)

	result, out, err := expectRegexWithOutput(child, "\n[0-9a-f-]{36}")
	if err != nil || len(result) != 1 {
		t.Fatalf("Error: %v\nOutput: %v", err, out)
	}

	podIDStr := strings.TrimSpace(result[0])
	podID, err := types.NewUUID(podIDStr)
	if err != nil {
		t.Fatalf("%q is not a valid UUID: %v", podIDStr, err)
	}

	return podID.String()
}

func runRktAsGidAndCheckOutput(t *testing.T, rktCmd, expectedLine string, expectError bool, gid int) {
	child, err := gexpect.Command(rktCmd)
	if err != nil {
		t.Fatalf("cannot exec rkt: %v", err)
	}
	if gid != 0 {
		child.Cmd.SysProcAttr = &syscall.SysProcAttr{}
		child.Cmd.SysProcAttr.Credential = &syscall.Credential{Uid: nobodyUid, Gid: uint32(gid)}
	}

	err = child.Start()
	if err != nil {
		t.Fatalf("cannot exec rkt: %v", err)
	}
	defer waitOrFail(t, child, !expectError)

	if expectedLine != "" {
		if err := expectWithOutput(child, expectedLine); err != nil {
			t.Fatalf("didn't receive expected output %q: %v", expectedLine, err)
		}
	}
}

func runRktAndCheckRegexOutput(t *testing.T, rktCmd, match string) {
	child := spawnOrFail(t, rktCmd)
	defer child.Wait()

	result, out, err := expectRegexWithOutput(child, match)
	if err != nil || len(result) != 1 {
		t.Fatalf("%q regex must be found one time, Error: %v\nOutput: %v", match, err, out)
	}
}

func runRktAndCheckOutput(t *testing.T, rktCmd, expectedLine string, expectError bool) {
	runRktAsGidAndCheckOutput(t, rktCmd, expectedLine, expectError, 0)
}

func checkAppStatus(t *testing.T, ctx *rktRunCtx, multiApps bool, appName, expected string) {
	cmd := fmt.Sprintf(`/bin/sh -c "`+
		`UUID=$(%s list --full|grep '%s'|awk '{print $1}') ;`+
		`echo -n 'status=' ;`+
		`%s status $UUID|grep '^app-%s.*=[0-9]*$'|cut -d= -f2"`,
		ctx.cmd(), appName, ctx.cmd(), appName)

	if multiApps {
		cmd = fmt.Sprintf(`/bin/sh -c "`+
			`UUID=$(%s list --full|grep '^[a-f0-9]'|awk '{print $1}') ;`+
			`echo -n 'status=' ;`+
			`%s status $UUID|grep '^app-%s.*=[0-9]*$'|cut -d= -f2"`,
			ctx.cmd(), ctx.cmd(), appName)
	}

	t.Logf("Get status for app %s\n", appName)
	child := spawnOrFail(t, cmd)
	defer waitOrFail(t, child, true)

	if err := expectWithOutput(child, expected); err != nil {
		// For debugging purposes, print the full output of
		// "rkt list" and "rkt status"
		cmd := fmt.Sprintf(`%s list --full ;`+
			`UUID=$(%s list --full|grep  '^[a-f0-9]'|awk '{print $1}') ;`+
			`%s status $UUID`,
			ctx.cmd(), ctx.cmd(), ctx.cmd())
		out, err2 := exec.Command("/bin/sh", "-c", cmd).CombinedOutput()
		if err2 != nil {
			t.Logf("Could not run rkt status: %v. %s", err2, out)
		} else {
			t.Logf("%s\n", out)
		}

		t.Fatalf("Failed to get the status for app %s: expected: %s. %v",
			appName, expected, err)
	}
}
