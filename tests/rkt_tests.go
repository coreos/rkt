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
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/ThomasRooney/gexpect"
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
	for _, d := range ctx.directories {
		d.reset()
	}
}

func (ctx *rktRunCtx) cleanup() {
	for _, d := range ctx.directories {
		d.cleanup()
	}
}

func (ctx *rktRunCtx) cmd() string {
	ctx.ensureValid()
	opts := make([]string, 0, len(ctx.directories))
	for _, d := range ctx.directories {
		opts = append(opts, d.rktOption())
	}
	rkt := os.Getenv("RKT")
	if rkt == "" {
		panic("Cannot run rkt: RKT env var is not specified")
	}
	abs, err := filepath.Abs(rkt)
	if err != nil {
		abs = rkt
	}
	return fmt.Sprintf("%s %s", abs, strings.Join(opts, " "))
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

func expectTimeoutWithOutput(p *gexpect.ExpectSubprocess, searchString string, timeout time.Duration) error {
	return expectCommon(p, searchString, timeout)
}

func patchTestACI(newFileName string, args ...string) string {
	var allArgs []string

	image := os.Getenv("RKT_INSPECT_IMAGE")
	if image == "" {
		panic("Cannot create ACI: RKT_INSPECT_IMAGE env var is not specified")
	}
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
	allArgs = append(allArgs, "--overwrite")
	allArgs = append(allArgs, args...)
	allArgs = append(allArgs, image)
	allArgs = append(allArgs, imagePath)

	output, err := exec.Command(actool, allArgs...).CombinedOutput()
	if err != nil {
		panic(fmt.Sprintf("Cannot create ACI: %v: %s\n", err, output))
	}
	return imagePath
}
