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
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/aci"
	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/schema"
	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/schema/types"
	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/opencontainers/specs"
	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/spf13/cobra"
	"github.com/coreos/rkt/store"
)

var (
	cmdImageImport = &cobra.Command{
		Use:   "import-oci OCI-BUNDLE",
		Short: "Convert imported oci-bundle to aci image",
		Long:  "Import an oci-bundle directory as input, convert to an aci image and store it in the local store",
		Run:   runWrapper(runImageImport),
	}
)

func init() {
	cmdImage.AddCommand(cmdImageImport)
}

func runImageImport(cmd *cobra.Command, args []string) (exit int) {
	if len(args) != 1 {
		cmd.Usage()
		return 1
	}

	aciImage, err := oci2aciImage(args[0])
	if err != nil {
		stderr("oci2aci failed: %s", err)
		return 1
	}

	//save aci to rkt store
	s, err := store.NewStore(globalFlags.Dir)
	if err != nil {
		fmt.Printf("cannot open store: %v", err)
		return 1
	}
	aciFile, err := os.Open(aciImage)
	if err != nil {
		fmt.Printf("error opening ACI file %s: %v", aciImage, err)
		return 1
	}
	key, err := s.WriteACI(aciFile, true)
	if err != nil {
		fmt.Printf("error write ACI file: %v", err)
		return 1
	}
	fmt.Println(key)
	return 0
}

func oci2aciImage(ociPath string) (string, error) {
	if bValidate := validateOCIProc(ociPath); bValidate != true {
		err := errors.New("Invalid oci bundle.")
		return "", err
	}

	dirWork := createWorkDir()
	if dirWork == "" {
		err := errors.New("Create working directory failed.")
		return "", err
	}
	// First, convert layout
	_, err := convertLayout(ociPath, dirWork)
	if err != nil {
		return "", err
	}

	// Second, build image
	aciImgPath, err := buildACI(dirWork)

	return aciImgPath, err

}

// Create work directory for the conversion output
func createWorkDir() string {
	idir, err := ioutil.TempDir("", "oci2aci")
	if err != nil {
		return ""
	}
	rootfs := filepath.Join(idir, "rootfs")
	os.MkdirAll(rootfs, 0755)

	data := []byte{}
	if err := ioutil.WriteFile(filepath.Join(idir, "manifest"), data, 0644); err != nil {
		return ""
	}
	return idir
}

type IsolatorCapSet struct {
	Sets []string `json:"set"`
}

type ResourceMem struct {
	Limit string `json:"limit"`
}

type ResourceCPU struct {
	Limit string `json:"limit"`
}

// The structure of appc manifest:
// 1.acKind
// 2. acVersion
// 3. name
// 4. labels
//	4.1 version
//	4.2 os
//	4.3 arch
// 5. app
//	5.1 exec
//	5.2 user
//	5.3 group
//	5.4 eventHandlers
//	5.5 workingDirectory
//	5.6 environment
//	5.7 mountPoints
//	5.8 ports
//      5.9 isolators
// 6. annotations
//	6.1 created
//	6.2 authors
//	6.3 homepage
//	6.4 documentation
// 7. dependencies
//	7.1 imageName
//	7.2 imageID
//	7.3 labels
//	7.4 size
// 8. pathWhitelist

func genManifest(path string) *schema.ImageManifest {
	// Get runtime.json and config.json
	runtimePath := path + "/runtime.json"
	configPath := path + "/config.json"

	runtime, err := ioutil.ReadFile(runtimePath)
	if err != nil {
		if globalFlags.Debug {
			stderr("Open file runtime.json failed:%v", err)
		}
		return nil
	}

	config, err := ioutil.ReadFile(configPath)
	if err != nil {
		if globalFlags.Debug {
			stderr("Open file config.json failed:%v", err)
		}
		return nil
	}

	var spec specs.LinuxSpec
	err = json.Unmarshal(config, &spec)
	if err != nil {
		if globalFlags.Debug {
			stderr("Unmarshal file config.json failed:%v", err)
		}

		return nil
	}

	var runSpec specs.LinuxRuntimeSpec
	err = json.Unmarshal(runtime, &runSpec)
	if err != nil {
		if globalFlags.Debug {
			stderr("Unmarshal file runtime.json failed:%v", err)
		}

		return nil
	}
	// Begin to convert runtime.json/config.json to manifest
	m := new(schema.ImageManifest)

	// 1. Assemble "acKind" field
	m.ACKind = "ImageManifest"

	// 2. Assemble "acVersion" field
	m.ACVersion = schema.AppContainerVersion

	// 3. Assemble "name" field
	m.Name = "oci"

	// 4. Assemble "labels" field
	// 4.1 "version"
	label := new(types.Label)
	label.Name = types.ACIdentifier("version")
	label.Value = spec.Version
	m.Labels = append(m.Labels, *label)
	// 4.2 "os"
	label = new(types.Label)
	label.Name = types.ACIdentifier("os")
	label.Value = spec.Platform.OS
	m.Labels = append(m.Labels, *label)
	// 4.3 "arch"
	label = new(types.Label)
	label.Name = types.ACIdentifier("arch")
	label.Value = spec.Platform.Arch
	m.Labels = append(m.Labels, *label)

	// 5. Assemble "app" field
	app := new(types.App)
	// 5.1 "exec"
	app.Exec = spec.Process.Args
	// 5.2 "user"
	app.User = fmt.Sprintf("%d", spec.Process.User.UID)
	// 5.3 "group"
	app.Group = fmt.Sprintf("%d", spec.Process.User.GID)
	// 5.4 "eventHandlers"
	event := new(types.EventHandler)
	event.Name = "pre-start"
	for index := range runSpec.Hooks.Prestart {
		event.Exec = append(event.Exec, runSpec.Hooks.Prestart[index].Path)
		event.Exec = append(event.Exec, runSpec.Hooks.Prestart[index].Args...)
		event.Exec = append(event.Exec, runSpec.Hooks.Prestart[index].Env...)
	}
	app.EventHandlers = append(app.EventHandlers, *event)
	event = new(types.EventHandler)
	event.Name = "post-stop"
	for index := range runSpec.Hooks.Poststop {
		event.Exec = append(event.Exec, runSpec.Hooks.Poststop[index].Path)
		event.Exec = append(event.Exec, runSpec.Hooks.Poststop[index].Args...)
		event.Exec = append(event.Exec, runSpec.Hooks.Poststop[index].Env...)
	}
	app.EventHandlers = append(app.EventHandlers, *event)
	// 5.5 "workingDirectory"
	app.WorkingDirectory = spec.Process.Cwd
	// 5.6 "environment"
	env := new(types.EnvironmentVariable)
	for index := range spec.Process.Env {
		s := strings.Split(spec.Process.Env[index], "=")
		env.Name = s[0]
		env.Value = s[1]
		app.Environment = append(app.Environment, *env)
	}

	// 5.7 "mountPoints"
	for index := range spec.Mounts {
		mount := new(types.MountPoint)
		mount.Name = types.ACName(spec.Mounts[index].Name)
		mount.Path = spec.Mounts[index].Path
		app.MountPoints = append(app.MountPoints, *mount)
	}

	// 5.8 "ports"

	// 5.9 "isolators"
	if runSpec.Linux.Resources != nil {
		if runSpec.Linux.Resources.CPU.Quota != 0 {
			cpuLimt := new(ResourceCPU)
			cpuLimt.Limit = fmt.Sprintf("%dm", runSpec.Linux.Resources.CPU.Quota)
			isolator := new(types.Isolator)
			isolator.Name = types.ACIdentifier("resource/cpu")
			bytes, _ := json.Marshal(cpuLimt)

			valueRaw := json.RawMessage(bytes)
			isolator.ValueRaw = &valueRaw

			app.Isolators = append(app.Isolators, *isolator)
		}
		if runSpec.Linux.Resources.Memory.Limit != 0 {
			memLimt := new(ResourceMem)
			memLimt.Limit = fmt.Sprintf("%dG", runSpec.Linux.Resources.Memory.Limit/(1024*1024*1024))
			isolator := new(types.Isolator)
			isolator.Name = types.ACIdentifier("resource/memory")
			bytes, _ := json.Marshal(memLimt)

			valueRaw := json.RawMessage(bytes)
			isolator.ValueRaw = &valueRaw

			app.Isolators = append(app.Isolators, *isolator)
		}
	}

	if len(spec.Linux.Capabilities) != 0 {
		isolatorCapSet := new(IsolatorCapSet)
		isolatorCapSet.Sets = append(isolatorCapSet.Sets, spec.Linux.Capabilities...)

		isolator := new(types.Isolator)
		isolator.Name = types.ACIdentifier(types.LinuxCapabilitiesRetainSetName)
		bytes, _ := json.Marshal(isolatorCapSet)

		valueRaw := json.RawMessage(bytes)
		isolator.ValueRaw = &valueRaw

		app.Isolators = append(app.Isolators, *isolator)
	}

	// 6. "annotations"

	// 7. "dependencies"

	// 8. "pathWhitelist"

	m.App = app

	return m
}

// Convert OCI layout to ACI layout
func convertLayout(srcPath, dstPath string) (string, error) {
	src, _ := filepath.Abs(srcPath)
	src += "/rootfs"
	if err := run(exec.Command("cp", "-rf", src, dstPath)); err != nil {
		return "", err
	}

	m := genManifest(srcPath)

	bytes, err := json.MarshalIndent(m, "", "\t")
	if err != nil {
		return "", err
	}

	manifestPath := dstPath + "/manifest"

	ioutil.WriteFile(manifestPath, bytes, 0644)
	return manifestPath, nil
}

func buildACI(dir string) (string, error) {
	imageName, err := filepath.Abs(dir)
	if err != nil {
		if globalFlags.Debug {
			stderr("err: %v", err)
		}
	}
	imageName += ".aci"
	err = createACI(dir, imageName)

	return imageName, err
}

func createACI(dir string, imageName string) error {
	var errStr string
	var errRes error
	buildNocompress := true
	root := dir
	tgt := imageName

	ext := filepath.Ext(tgt)
	if ext != schema.ACIExtension {
		errStr = fmt.Sprintf("build: Extension must be %s (given %s)", schema.ACIExtension, ext)
		errRes = errors.New(errStr)
		return errRes
	}

	if err := aci.ValidateLayout(root); err != nil {
		if e, ok := err.(aci.ErrOldVersion); ok {
			if globalFlags.Debug {
				stderr("build: Warning: %v. Please update your manifest.", e)
			}
		} else {
			errStr = fmt.Sprintf("build: Layout failed validation: %v", err)
			errRes = errors.New(errStr)
			return errRes
		}
	}

	mode := os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	fh, err := os.OpenFile(tgt, mode, 0644)
	if err != nil {
		errStr = fmt.Sprintf("build: Unable to open target %s: %v", tgt, err)
		errRes = errors.New(errStr)
		return errRes
	}

	var gw *gzip.Writer
	var r io.WriteCloser = fh
	if !buildNocompress {
		gw = gzip.NewWriter(fh)
		r = gw
	}
	tr := tar.NewWriter(r)

	defer func() {
		tr.Close()
		if !buildNocompress {
			gw.Close()
		}
		fh.Close()
	}()

	mpath := filepath.Join(root, aci.ManifestFile)
	b, err := ioutil.ReadFile(mpath)
	if err != nil {
		errStr = fmt.Sprintf("build: Unable to read Image Manifest: %v", err)
		errRes = errors.New(errStr)
		return errRes
	}
	var im schema.ImageManifest
	if err := im.UnmarshalJSON(b); err != nil {
		errStr = fmt.Sprintf("build: Unable to load Image Manifest: %v", err)
		errRes = errors.New(errStr)
		return errRes
	}
	iw := aci.NewImageWriter(im, tr)

	err = filepath.Walk(root, aci.BuildWalker(root, iw))
	if err != nil {
		errStr = fmt.Sprintf("build: Error walking rootfs: %v", err)
		errRes = errors.New(errStr)
		return errRes
	}

	err = iw.Close()
	if err != nil {
		errStr = fmt.Sprintf("build: Unable to close image %s: %v", tgt, err)
		errRes = errors.New(errStr)
		return errRes
	}

	return nil
}

const (
	// Path to config file inside the bundle
	ConfigFile  = "config.json"
	RuntimeFile = "runtime.json"
	// Path to rootfs directory inside the bundle
	RootfsDir = "rootfs"
)

var (
	ErrNoRootFS = errors.New("no rootfs found in bundle")
	ErrNoConfig = errors.New("no config json file found in bundle")
	ErrNoRun    = errors.New("no runtime json file found in bundle")
)

type validateRes struct {
	cfgOK   bool
	runOK   bool
	rfsOK   bool
	config  io.Reader
	runtime io.Reader
}

func validateOCIProc(path string) bool {
	var bRes bool
	err := validateBundle(path)
	if err != nil {
		if globalFlags.Debug {
			stderr("%s: invalid oci bundle: %v\n", path, err)
		}
		bRes = false
	} else {
		if globalFlags.Debug {
			stderr("%s: valid oci bundle\n", path)
		}
		bRes = true
	}
	return bRes
}

func validateBundle(path string) error {
	fi, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("error accessing bundle: %v", err)
	}
	if !fi.IsDir() {
		return fmt.Errorf("given path %q is not a directory", path)
	}
	var flist []string
	var res validateRes
	walkBundle := func(fpath string, fi os.FileInfo, err error) error {
		rpath, err := filepath.Rel(path, fpath)
		if err != nil {
			return err
		}
		switch rpath {
		case ".":
		case ConfigFile:
			res.config, err = os.Open(fpath)
			if err != nil {
				return err
			}
			res.cfgOK = true
		case RuntimeFile:
			res.runtime, err = os.Open(fpath)
			if err != nil {
				return err
			}
			res.runOK = true
		case RootfsDir:
			if !fi.IsDir() {
				return errors.New("rootfs is not a directory")
			}
			res.rfsOK = true
		default:
			flist = append(flist, rpath)
		}
		return nil
	}
	if err := filepath.Walk(path, walkBundle); err != nil {
		return err
	}
	return checkBundle(res, flist)
}

func checkBundle(res validateRes, files []string) error {
	defer func() {
		if rc, ok := res.config.(io.Closer); ok {
			rc.Close()
		}
		if rc, ok := res.runtime.(io.Closer); ok {
			rc.Close()
		}
	}()
	if !res.cfgOK {
		return ErrNoConfig
	}
	if !res.runOK {
		return ErrNoRun
	}
	if !res.rfsOK {
		return ErrNoRootFS
	}
	_, err := ioutil.ReadAll(res.config)
	if err != nil {
		return fmt.Errorf("error reading the bundle: %v", err)
	}
	_, err = ioutil.ReadAll(res.runtime)
	if err != nil {
		return fmt.Errorf("error reading the bundle: %v", err)
	}

	for _, f := range files {
		if !strings.HasPrefix(f, "rootfs") {
			return fmt.Errorf("unrecognized file path in bundle: %q", f)
		}
	}
	return nil
}

type Err struct {
	Message string
	File    string
	Path    string
	Func    string
	Line    int
}

func (e *Err) Error() string {
	return fmt.Sprintf("[%v:%v] %v", e.File, e.Line, e.Message)
}

func run(cmd *exec.Cmd) error {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return errorf(err.Error())
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return errorf(err.Error())
	}
	go io.Copy(os.Stdout, stdout)
	go io.Copy(os.Stderr, stderr)
	return cmd.Run()
}

func errorf(format string, args ...interface{}) error {
	msg := fmt.Sprintf(format, args...)
	pc, filePath, lineNo, ok := runtime.Caller(1)
	if !ok {
		return &Err{
			Message: msg,
			File:    "unknown_file",
			Path:    "unknown_path",
			Func:    "unknown_func",
			Line:    0,
		}
	}
	return &Err{
		Message: msg,
		File:    filepath.Base(filePath),
		Path:    filePath,
		Func:    runtime.FuncForPC(pc).Name(),
		Line:    lineNo,
	}
}
