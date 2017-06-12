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

package networking

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/containernetworking/cni/libcni"
	"github.com/containernetworking/cni/pkg/invoke"
	cnitypes "github.com/containernetworking/cni/pkg/types"
	cniv031 "github.com/containernetworking/cni/pkg/types/current"
	"github.com/hashicorp/errwrap"

	"github.com/rkt/rkt/common"
)

// TODO(eyakubovich): make this configurable in rkt.conf
const UserNetPluginsPath = "/usr/lib/rkt/plugins/net"
const BuiltinNetPluginsPath = "usr/lib/rkt/plugins/net"

func pluginErr(err error, output []byte) error {
	if _, ok := err.(*exec.ExitError); ok {
		emsg := cnitypes.Error{}
		if perr := json.Unmarshal(output, &emsg); perr != nil {
			return errwrap.Wrap(fmt.Errorf("netplugin failed but error parsing its diagnostic message %q", string(output)), perr)
		}
		details := ""
		if emsg.Details != "" {
			details = fmt.Sprintf("; %v", emsg.Details)
		}
		return fmt.Errorf("%v%v", emsg.Msg, details)
	}

	return err
}

func (e *podEnv) cniInfo(n *activeNet) (*libcni.RuntimeConf, libcni.CNIConfig) {
	return &libcni.RuntimeConf{
			ContainerID: e.podID.String(),
			NetNS:       e.podNS.Path(),
			IfName:      n.runtime.IfName,
			Args:        n.runtime.Args,
		}, libcni.CNIConfig{
			Path: e.pluginPaths(),
		}
}

// Executes a given network plugin. If successful, mutates n.runtime with
// the runtime information
func (e *podEnv) netPluginAdd(n *activeNet, netns string) error {
	rtc, cnc := e.cniInfo(n)
	resultI, err := cnc.AddNetworkList(&n.conf, rtc)
	if err != nil {
		return err
	}
	result, err := cniv031.NewResultFromResult(resultI)
	if err != nil {
		return err
	}

	// All is well - mutate the runtime
	n.runtime.MergeCNIResult(result)
	return nil
}

func (e *podEnv) netPluginDel(n *activeNet, netns string) error {
	rtc, cnc := e.cniInfo(n)
	if err := cnc.DelNetworkList(&n.conf, rtc); err != nil {
		return err
	}
	return nil
}

func (e *podEnv) pluginPaths() []string {
	// try 3rd-party path first
	return []string{
		filepath.Join(e.localConfig, UserNetPathSuffix),
		UserNetPluginsPath,
		filepath.Join(common.Stage1RootfsPath(e.podRoot), BuiltinNetPluginsPath),
	}
}

func (e *podEnv) findNetPlugin(plugin string) string {
	for _, p := range e.pluginPaths() {
		fullname := filepath.Join(p, plugin)
		if fi, err := os.Stat(fullname); err == nil && fi.Mode().IsRegular() {
			return fullname
		}
	}

	return ""
}

func envVars(vars [][2]string) []string {
	env := os.Environ()

	for _, kv := range vars {
		env = append(env, strings.Join(kv[:], "="))
	}

	return env
}

// Big time hack, only used for KVM executing the IPAM plugin
func (e *podEnv) execNetPlugin(cmd string, n *activeNet, pluginName string, confBytes []byte) (cnitypes.Result, error) {
	args := invoke.Args{
		Command:     cmd,
		ContainerID: e.podID.String(),
		NetNS:       e.podNS.Path(),
		PluginArgs:  n.runtime.Args,
		IfName:      n.runtime.IfName,
		Path:        strings.Join(e.pluginPaths(), ":"),
	}

	pluginPath, err := invoke.FindInPath(pluginName, e.pluginPaths())
	if err != nil {
		return nil, err
	}
	return invoke.ExecPluginWithResult(pluginPath, confBytes, &args)
}
