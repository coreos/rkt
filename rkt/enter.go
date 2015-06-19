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
		Use:   "enter [--app=APPNAME] UUID [CMD [ARGS ...]]",
		Short: "Enter the namespaces of an app within a rkt pod",
		Run:   runWrapper(runEnter),
	}
	flagAppName string
)

const (
	defaultCmd = "/bin/bash"
)

func init() {
	cmdRkt.AddCommand(cmdEnter)
	cmdEnter.Flags().StringVar(&flagAppName, "app", "", "name of the app to enter within the specified pod")

	// Disable interspersed flags to stop parsing after the first non flag
	// argument. This is need to permit to correctly handle
	// multiple "IMAGE -- imageargs ---"  options
	cmdEnter.Flags().SetInterspersed(false)
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
		stderr("Unable to determine the pid for pod %q: %v", pid, err)
		return 1
	}

	index, err := getAppIndex(p)
	if err != nil {
		stderr("Unable to determine the app index: %v", err)
		return 1
	}

	argv, err := getEnterArgv(p, args)
	if err != nil {
		stderr("Enter failed: %v", err)
		return 1
	}

	s, err := store.NewStore(globalFlags.Dir)
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

	if err = stage0.Enter(p.path(), podPID, index, stage1RootFS, argv); err != nil {
		stderr("Enter failed: %v", err)
		return 1
	}
	// not reached when stage0.Enter execs /enter
	return 0
}

// getAppIndex returns the position of the app within the pod. This is used
// for entering the app's namespace later.
//
// If flagAppName is supplied in the flags then we will read the pod manifest and
// compute the position.
// If the PM contains a single app, that app's position("0" in this case) is returned.
// If the PM has multiple apps, the names are printed and an error is returned.
func getAppIndex(p *pod) (int, error) {
	var appName *types.ACName
	var err error

	if flagAppName != "" {
		appName, err = types.NewACName(flagAppName)
		if err != nil {
			return -1, err
		}
	}

	// figure out the image id, or show a list if multiple are present
	b, err := ioutil.ReadFile(common.PodManifestPath(p.path()))
	if err != nil {
		return -1, fmt.Errorf("error reading pod manifest: %v", err)
	}

	m := schema.PodManifest{}
	if err = m.UnmarshalJSON(b); err != nil {
		return -1, fmt.Errorf("unable to load manifest: %v", err)
	}

	switch len(m.Apps) {
	case 0:
		return -1, fmt.Errorf("pod contains zero apps")
	case 1:
		return 0, nil
	default:
	}

	if appName != nil {
		for index, app := range m.Apps {
			if app.Name.Equals(*appName) {
				return index, nil
			}
		}
		return -1, fmt.Errorf("app %q is not defined in the pod", appName.String())
	}

	stderr("Pod contains multiple apps:")
	for _, ra := range m.Apps {
		stderr("\t%s", ra.Name.String())
	}

	return -1, fmt.Errorf("specify app using \"rkt enter --app= ...\"")
}

// getEnterArgv returns the argv to use for entering the pod
func getEnterArgv(p *pod, cmdArgs []string) ([]string, error) {
	var argv []string
	if len(cmdArgs) < 2 {
		stderr("No command specified, assuming %q", defaultCmd)
		argv = []string{defaultCmd}
	} else {
		argv = cmdArgs[1:]
	}

	return argv, nil
}
