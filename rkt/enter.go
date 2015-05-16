// Copyright 2014 The rkt Authors
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

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/schema"
	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/schema/types"
	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/spf13/cobra"
	"github.com/coreos/rkt/common"
	"github.com/coreos/rkt/stage0"
	"github.com/coreos/rkt/store"
)

var (
	cmdEnter = &cobra.Command{
		Use:   "enter [--imageid IMAGEID] UUID -- [CMD [ARGS ...]]",
		Short: "Enter the namespaces of an app within a rkt pod",
		Run: func(cmd *cobra.Command, args []string) {
			subCmdExitCode = runEnter(cmd, args)
		},
	}
	flagImageID string
)

const (
	defaultCmd = "/bin/bash"
)

func init() {
	cmdEnter.Flags().StringVarP(&flagImageID, "imageid", "", "", "imageid of the app to enter within the specified pod")
	rktCmd.AddCommand(cmdEnter)
}

func runEnter(cmd *cobra.Command, args []string) (exit int) {

	if len(args) < 1 {
		cmd.Usage()
		return 1
	}

	podUUID, err := resolveUUID(args[0])
	if err != nil {
		stderr("Unable to resolve UUID: %v", err)
		return 1
	}

	pid := podUUID.String()
	p, err := getPod(pid)
	if err != nil {
		stderr("Failed to open pod %q: %v", pid, err)
		return 1
	}
	defer p.Close()

	if !p.isRunning() {
		stderr("Pod %q isn't currently running", pid)
		return 1
	}

	podPID, err := p.getPID()
	if err != nil {
		stderr("Unable to determine pid for pod %q: %v", pid, err)
		return 1
	}

	imageID, err := getAppImageID(p)
	if err != nil {
		stderr("Unable to determine image id: %v", err)
		return 1
	}

	argv, err := getEnterArgv(p, imageID, args)
	if err != nil {
		stderr("Enter failed: %v", err)
		return 1
	}

	s, err := store.NewStore(flagDataDir)
	if err != nil {
		stderr("Cannot open store: %v", err)
		return 1
	}

	stage1ID, err := p.getStage1Hash()
	if err != nil {
		stderr("Error getting stage1 hash")
		return 1
	}

	stage1RootFS := s.GetTreeStoreRootFS(stage1ID.String())

	if err = stage0.Enter(p.path(), podPID, imageID, stage1RootFS, argv); err != nil {
		stderr("Enter failed: %v", err)
		return 1
	}
	// not reached when stage0.Enter execs /enter
	return 0
}

// getAppImageID returns the image id to enter
// If one was supplied in the flags then it's simply returned
// If the PM contains a single image, that image's id is returned
// If the PM has multiple images, the ids and names are printed and an error is returned
func getAppImageID(p *pod) (*types.Hash, error) {
	if len(flagImageID) > 0 {
		imageID, err := types.NewHash(flagImageID)
		return imageID, err
	}

	// figure out the image id, or show a list if multiple are present
	b, err := ioutil.ReadFile(common.PodManifestPath(p.path()))
	if err != nil {
		return nil, fmt.Errorf("error reading pod manifest: %v", err)
	}

	m := schema.PodManifest{}
	if err = m.UnmarshalJSON(b); err != nil {
		return nil, fmt.Errorf("unable to load manifest: %v", err)
	}

	switch len(m.Apps) {
	case 0:
		return nil, fmt.Errorf("pod contains zero apps")
	case 1:
		return &m.Apps[0].Image.ID, nil
	default:
	}

	stderr("Pod contains multiple apps:")
	for _, ra := range m.Apps {
		stderr("\t%s: %s", types.ShortHash(ra.Image.ID.String()), ra.Name.String())
	}

	return nil, fmt.Errorf("specify app using \"rkt enter --imageid ...\"")
}

// getEnterArgv returns the argv to use for entering the pod
func getEnterArgv(p *pod, imageID *types.Hash, cmdArgs []string) ([]string, error) {
	var argv []string
	if len(cmdArgs) < 2 {
		stdout("No command specified, assuming %q", defaultCmd)
		argv = []string{defaultCmd}
	} else {
		argv = cmdArgs[1:]
	}

	return argv, nil
}
