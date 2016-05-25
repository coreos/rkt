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

package oci2aci

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/schema"
	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/schema/types"
)

type IsolatorCapSet struct {
	Sets []string `json:"set"`
}

type ResourceMem struct {
	Limit string `json:"limit"`
}

type ResourceCPU struct {
	Limit string `json:"limit"`
}

func Oci2aciManifest(ociPath string) (string, error) {
	if bValidate := validateOCIProc(ociPath); bValidate != true {
		err := errors.New("Invalid oci bundle.")
		return "", err
	}

	dirWork := createWorkDir()
	// convert layout
	aciManifestPath, err := convertLayout(ociPath, dirWork)
	if err != nil {
		return "", err
	}
	return aciManifestPath, err

}

func Oci2aciImage(ociPath string) (string, error) {
	if bValidate := validateOCIProc(ociPath); bValidate != true {
		err := errors.New("Invalid oci bundle.")
		return "", err
	}

	dirWork := createWorkDir()
	// First, convert layout
	_, err := convertLayout(ociPath, dirWork)
	if err != nil {
		return "", err
	}

	// Second, build image
	aciImgPath, err := buildACI(dirWork)

	return aciImgPath, err

}

// Entry point of oci2aci,
// First convert oci layout to aci layout, then build aci layout to image.
func RunOCI2ACI(path string, flagDebug bool) error {
	if flagDebug {
		InitDebug()
	}
	if bValidate := validateOCIProc(path); bValidate != true {
		log.Printf("Conversion stop.")
		return nil
	}

	dirWork := createWorkDir()
	// First, convert layout
	manifestPath, err := convertLayout(path, dirWork)
	if err != nil {
		if debugEnabled {
			log.Printf("Conversion from oci to aci layout failed: %v", err)
		}

	} else {
		if debugEnabled {
			log.Printf("Manifest:%v generated successfully.", manifestPath)
		}
	}
	// Second, build image
	imgPath, err := buildACI(dirWork)
	if err != nil {
		if debugEnabled {
			log.Printf("Generate aci image failed:%v", err)
		}
	} else {
		if debugEnabled {
			log.Printf("Image:%v generated successfully.", imgPath)
		}
	}

	return nil
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
		if debugEnabled {
			log.Printf("Open file runtime.json failed: %v", err)
		}
		return nil
	}

	config, err := ioutil.ReadFile(configPath)
	if err != nil {
		if debugEnabled {
			log.Printf("Open file config.json failed: %v", err)
		}
		return nil
	}

	var spec LinuxSpec
	err = json.Unmarshal(config, &spec)
	if err != nil {
		if debugEnabled {
			log.Printf("Unmarshal config.json failed: %v", err)
		}
		return nil
	}

	var runSpec LinuxRuntimeSpec
	err = json.Unmarshal(runtime, &runSpec)
	if err != nil {
		if debugEnabled {
			log.Printf("Unmarshal runtime.json failed: %v", err)
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
		//mount.ReadOnly = true
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
