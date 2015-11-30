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

package aci

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type aciToolkit struct {
	acTool string
	goTool string
}

func (t *aciToolkit) prepareACI() ([]byte, error) {
	tmp := os.Getenv("FUNCTIONAL_TMP")
	if tmp == "" {
		return nil, fmt.Errorf("empty FUNCTIONAL_TMP env var")
	}
	taas, dir, err := t.createTree(tmp)
	if taas != "" {
		defer os.RemoveAll(taas)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to build ACI tree: %v", err)
	}
	if err := t.buildProg(dir); err != nil {
		return nil, fmt.Errorf("failed to build test program: %v", err)
	}
	fn, err := t.buildACI(tmp, dir)
	if err != nil {
		return nil, fmt.Errorf("failed to build ACI: %v", err)
	}
	defer os.Remove(fn)
	contents, err := ioutil.ReadFile(fn)
	if err != nil {
		return nil, fmt.Errorf("failed to read ACI to memory: %v", err)
	}
	return contents, nil
}

const (
	manifestStr    = `{"acKind":"ImageManifest","acVersion":"0.7.3","name":"testprog","app":{"exec":["/prog"],"user":"0","group":"0"},"labels":[{"name":"os","value":"linux"},{"name":"arch","value":"amd64"}]}`
	testProgSrcStr = `
package main

import "fmt"

func main() {
	fmt.Println("Authentication succeeded.")
}
`
)

func (t *aciToolkit) createTree(tmpDir string) (string, string, error) {
	taasDir := filepath.Join(tmpDir, "taas")
	aciDir := filepath.Join(taasDir, "ACI")
	rootDir := filepath.Join(aciDir, "rootfs")
	manifestFile := filepath.Join(aciDir, "manifest")
	srcFile := filepath.Join(rootDir, "prog.go")
	if err := os.Mkdir(taasDir, 0755); err != nil {
		return "", "", fmt.Errorf("failed to create taas directory: %v", err)
	}
	if err := os.Mkdir(aciDir, 0755); err != nil {
		return taasDir, "", fmt.Errorf("failed to create ACI directory: %v", err)
	}
	if err := os.Mkdir(rootDir, 0755); err != nil {
		return taasDir, "", fmt.Errorf("failed to create rootfs directory: %v", err)
	}
	if err := ioutil.WriteFile(manifestFile, []byte(manifestStr), 0644); err != nil {
		return taasDir, "", fmt.Errorf("failed to write manifest: %v", err)
	}
	if err := ioutil.WriteFile(srcFile, []byte(testProgSrcStr), 0644); err != nil {
		return taasDir, "", fmt.Errorf("failed to write go source: %v", err)
	}
	return taasDir, aciDir, nil
}

func (t *aciToolkit) buildProg(aciDir string) error {
	args := []string{
		"go",
		"build",
		"-o",
		"prog",
		"./prog.go",
	}
	dir := filepath.Join(aciDir, "rootfs")
	return runTool(t.goTool, args, dir)
}

func (t *aciToolkit) buildACI(tmpDir, aciDir string) (string, error) {
	timedata, err := time.Now().MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("failed to serialize current date to bytes: %v", err)
	}
	if err := ioutil.WriteFile(filepath.Join(aciDir, "rootfs", "stamp"), timedata, 0644); err != nil {
		return "", fmt.Errorf("failed to write a stamp: %v", err)
	}
	fn := filepath.Join(tmpDir, "prog-build.aci")
	args := []string{
		"actool",
		"build",
		aciDir,
		fn,
	}
	if err := runTool(t.acTool, args, ""); err != nil {
		return "", err
	}
	return fn, nil
}

func runTool(tool string, args []string, dir string) error {
	outBuf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cmd := exec.Cmd{
		Path:   tool,
		Args:   args,
		Dir:    dir,
		Stdout: outBuf,
		Stderr: errBuf,
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to execute `%s %s`: %v\nstdout:\n%v\n\nstderr:\n%v)", args[0], args[1], err, outBuf.String(), errBuf.String())
	}
	return nil
}
