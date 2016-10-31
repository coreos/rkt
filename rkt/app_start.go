// Copyright 2016 The rkt Authors
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

	pkgPod "github.com/coreos/rkt/pkg/pod"
	"github.com/coreos/rkt/stage0"

	"github.com/appc/spec/schema/types"
	"github.com/spf13/cobra"
)

var (
	cmdAppStart = &cobra.Command{
		Use:   "start UUID --app=NAME",
		Short: "Start an app in a pod",
		Long:  `Start appz!`,
		Run:   runWrapper(runAppStart),
	}
)

func init() {
	cmdAppStart.Flags().StringVar(&flagAppName, "app", "", "app to start")
	cmdApp.AddCommand(cmdAppStart)
}

func runAppStart(cmd *cobra.Command, args []string) (exit int) {
	if len(args) < 1 {
		stderr.Print("must provide the pod UUID")
		return 1
	}

	if flagAppName == "" {
		stderr.Print("must provide the app to start")
		return 1
	}

	p, err := pkgPod.PodFromUUIDString(getDataDir(), args[0])
	if err != nil {
		stderr.PrintE("problem retrieving pod", err)
		return 1
	}
	defer p.Close()

	if p.State() != pkgPod.Running {
		stderr.Printf("pod %q isn't currently running", p.UUID)
		return 1
	}

	appName, err := types.NewACName(flagAppName)
	if err != nil {
		stderr.PrintE("invalid app name", err)
	}

	podPID, err := p.ContainerPid1()
	if err != nil {
		stderr.PrintE(fmt.Sprintf("unable to determine the pid for pod %q", p.UUID), err)
		return 1
	}

	cfg := stage0.CommonConfig{
		UUID:  p.UUID,
		Debug: globalFlags.Debug,
	}

	scfg := stage0.StartConfig{
		CommonConfig: &cfg,
		Dir:          p.Path(),
		AppName:      appName,
		PodPID:       podPID,
	}

	err = stage0.StartApp(scfg)
	if err != nil {
		stderr.PrintE("error starting app", err)
		return 1
	}

	return 0
}
