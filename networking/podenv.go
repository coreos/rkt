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
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/appc/spec/schema/types"
	"github.com/hashicorp/errwrap"

	"github.com/coreos/rkt/common"
	"github.com/coreos/rkt/networking/config"
	nettypes "github.com/coreos/rkt/networking/types"
)

const (
	// Suffix to LocalConfigDir path, where users place their net configs
	UserNetPathSuffix = "net.d"

	// Default net path relative to stage1 root
	DefaultNetPath           = "etc/rkt/net.d/99-default.conf"
	DefaultRestrictedNetPath = "etc/rkt/net.d/99-default-restricted.conf"
)

// "base" struct that's populated from the beginning
// describing the environment in which the pod
// is running in
type podEnv struct {
	podRoot      string
	podID        types.UUID
	netsLoadList common.NetList
	localConfig  string
	config       *config.Config
}

// Loads nets specified by user and default one from stage1
func (e *podEnv) loadNets() ([]*nettypes.ActiveNet, error) {
	if e.netsLoadList.None() {
		stderr.Printf("networking namespace with loopback only")
		return nil, nil
	}

	var nets []*nettypes.ActiveNet
	overridesDefault := false
	for _, n := range e.config.Networks.Ordered {
		if !(e.netsLoadList.All() || e.netsLoadList.Specific(n.Conf.Name)) {
			continue
		}
		if n.Conf.Name == "default" || n.Conf.Name == "default-restricted" {
			overridesDefault = true
			stderr.Printf(`overriding %q network with %v`, n.Conf.Name, n.Runtime.ConfPath)
		}
		n.Runtime.Args = e.netsLoadList.SpecificArgs(n.Conf.Name)
		nets = append(nets, n)
	}

	if !overridesDefault {
		defPath := path.Join(common.Stage1RootfsPath(e.podRoot), "etc", "rkt")
		defCfg, err := config.GetConfigFrom(defPath)
		if err != nil {
			return nil, err
		}
		var name string
		if e.netsLoadList.Specific("default") || e.netsLoadList.All() {
			name = "default"
		} else {
			name = "default-restricted"
		}
		if an, exists := defCfg.Networks.ByName[name]; exists {
			nets = append(nets, an)
		} else {
			return nil, fmt.Errorf("No configuration for %q network in stage1", name)
		}
	}

	missing := e.missingNets(nets)
	if len(missing) > 0 {
		return nil, fmt.Errorf("networks not found: %v", strings.Join(missing, ", "))
	}

	return nets, nil
}

func (e *podEnv) podNSPath() string {
	return filepath.Join(e.podRoot, "netns")
}

func (e *podEnv) netDir() string {
	return filepath.Join(e.podRoot, "net")
}

func (e *podEnv) setupNets(nets []*nettypes.ActiveNet) error {
	err := os.MkdirAll(e.netDir(), 0755)
	if err != nil {
		return err
	}

	i := 0
	defer func() {
		if err != nil {
			e.teardownNets(nets[:i])
		}
	}()

	nspath := e.podNSPath()

	var n *nettypes.ActiveNet
	for i, n = range nets {
		stderr.Printf("loading network %v with type %v", n.Conf.Name, n.Conf.Type)

		n.Runtime.IfName = fmt.Sprintf(IfNamePattern, i)
		if n.Runtime.ConfPath, err = e.putConfInNetDir(n); err != nil {
			return err
		}

		n.Runtime.IP, n.Runtime.HostIP, err = e.netPluginAdd(n, nspath)
		if err != nil {
			return errwrap.Wrap(fmt.Errorf("error adding network %q", n.Conf.Name), err)
		}
	}
	return nil
}

func (e *podEnv) teardownNets(nets []*nettypes.ActiveNet) {
	nspath := e.podNSPath()

	for i := len(nets) - 1; i >= 0; i-- {
		stderr.Printf("teardown - executing net-plugin %v", nets[i].Conf.Type)

		err := e.netPluginDel(nets[i], nspath)
		if err != nil {
			stderr.PrintE(fmt.Sprintf("error deleting %q", nets[i].Conf.Name), err)
		}

		// Delete the conf file to signal that the network was
		// torn down (or at least attempted to)
		if err = os.Remove(nets[i].Runtime.ConfPath); err != nil {
			stderr.PrintE(fmt.Sprintf("error deleting %q", nets[i].Runtime.ConfPath), err)
		}
	}
}

func (e *podEnv) putConfInNetDir(n *nettypes.ActiveNet) (string, error) {
	dst := filepath.Join(e.netDir(), filepath.Base(n.Runtime.ConfPath))
	if err := ioutil.WriteFile(dst, n.ConfBytes, 0666); err != nil {
		return "", errwrap.Wrap(fmt.Errorf("error writing the config in %q", dst), err)
	}
	return dst, nil
}

func (e *podEnv) missingNets(loaded []*nettypes.ActiveNet) []string {
	diff := make(map[string]struct{})
	for _, n := range e.netsLoadList.StringsOnlyNames() {
		if n != "all" {
			diff[n] = struct{}{}
		}
	}

	for _, an := range loaded {
		delete(diff, an.Conf.Name)
	}

	var missing []string
	for n, _ := range diff {
		missing = append(missing, n)
	}
	return missing
}
