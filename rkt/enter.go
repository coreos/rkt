// Copyright 2014 CoreOS, Inc.
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

//+build linux

package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/coreos/rocket/Godeps/_workspace/src/github.com/appc/spec/schema"
	"github.com/coreos/rocket/Godeps/_workspace/src/github.com/appc/spec/schema/types"
	"github.com/coreos/rocket/common"
	"github.com/coreos/rocket/stage0"
)

var (
	cmdEnter = &Command{
		Name:    cmdEnterName,
		Summary: "Enter the namespaces of an app within a rkt container",
		Usage:   "[--imageid IMAGEID] UUID [CMD [ARGS ...]]",
		Run:     runEnter,
	}
	flagAppImageID types.Hash
)

const (
	defaultCmd   = "/bin/bash"
	cmdEnterName = "enter"
)

func init() {
	commands = append(commands, cmdEnter)
	cmdEnter.Flags.Var(&flagAppImageID, "imageid", "imageid of the app to enter within the specified container")
}

func runEnter(args []string) (exit int) {

	if len(args) < 1 {
		printCommandUsageByName(cmdEnterName)
		return 1
	}

	containerUUID, err := resolveUUID(args[0])
	if err != nil {
		stderr("Unable to resolve UUID: %v", err)
		return 1
	}

	cid := containerUUID.String()
	c, err := getContainer(cid)
	if err != nil {
		stderr("Failed to open container %q: %v", cid, err)
		return 1
	}
	defer c.Close()

	if !c.isRunning() {
		stderr("Container %q isn't currently running", cid)
		return 1
	}

	imageID, err := getAppImageID(c)
	if err != nil {
		stderr("Unable to determine image id: %v", err)
		return 1
	}

	if _, err = os.Stat(filepath.Join(common.AppRootfsPath(c.path(), *imageID))); err != nil {
		stderr("Unable to access app rootfs: %v", err)
		return 1
	}

	argv, err := getEnterArgv(c, imageID, args)
	if err != nil {
		stderr("Enter failed: %v", err)
		return 1
	}

	if err = stage0.Enter(c.path(), imageID, argv); err != nil {
		stderr("Enter failed: %v", err)
		return 1
	}
	// not reached when stage0.Enter execs /enter
	return 0
}

// getAppImageID returns the image id to enter
// If one was supplied in the flags then it's simply returned
// If the CRM contains a single image, that image's id is returned
// If the CRM has multiple images, the ids and names are printed and an error is returned
func getAppImageID(c *container) (*types.Hash, error) {
	if !flagAppImageID.Empty() {
		return &flagAppImageID, nil
	}

	// figure out the image id, or show a list if multiple are present
	b, err := ioutil.ReadFile(common.ContainerManifestPath(c.path()))
	if err != nil {
		return nil, fmt.Errorf("error reading container manifest: %v", err)
	}

	m := schema.ContainerRuntimeManifest{}
	if err = m.UnmarshalJSON(b); err != nil {
		return nil, fmt.Errorf("unable to load manifest: %v", err)
	}

	switch len(m.Apps) {
	case 0:
		return nil, fmt.Errorf("container contains zero apps")
	case 1:
		return &m.Apps[0].Image.ID, nil
	default:
	}

	stderr("Container contains multiple apps:")
	for _, ra := range m.Apps {
		stderr("\t%s: %s", types.ShortHash(ra.Image.ID.String()), ra.Name.String())
	}

	return nil, fmt.Errorf("specify app using \"rkt enter --imageid ...\"")
}

// getEnterArgv returns the argv to use for entering the container
func getEnterArgv(c *container, imageID *types.Hash, cmdArgs []string) ([]string, error) {
	var argv []string
	if len(cmdArgs) < 2 {
		stdout("No command specified, assuming %q", defaultCmd)
		argv = []string{defaultCmd}
	} else {
		argv = cmdArgs[1:]
	}

	// TODO(vc): LookPath() uses os.Stat() internally so symlinks can defeat this check
	if _, err := exec.LookPath(filepath.Join(common.AppRootfsPath(c.path(), *imageID), argv[0])); err != nil {
		return nil, fmt.Errorf("command %q missing, giving up: %v", argv[0], err)
	}

	return argv, nil
}
