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
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/cni/pkg/plugin"
	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/schema/types"

	"github.com/coreos/rkt/common"
	"github.com/coreos/rkt/networking/netinfo"
)

const (
	// Absolute path where users place their net configs
	UserNetPath = "/etc/rkt/net.d"

	// Default net path relative to stage1 root
	DefaultNetPath = "etc/rkt/net.d/99-default.conf"
)

// "base" struct that's populated from the beginning
// describing the environment in which the pod
// is running in
type podEnv struct {
	podRoot string
	podID   types.UUID
}

type activeNet struct {
	confBytes []byte
	conf      *plugin.NetConf
	runtime   *netinfo.NetInfo
	hostIP    net.IP // kludge for default network
}

// Loads nets specified by user and default one from stage1
func (e *Networking) loadNets() ([]activeNet, error) {
	nets, err := loadUserNets(e.netsLoadList)
	if err != nil {
		return nil, err
	}

	if !netExists(nets, "default") && (e.netsLoadList["default"] || e.netsLoadList["true"]) {
		defPath := path.Join(common.Stage1RootfsPath(e.podRoot), DefaultNetPath)
		n, err := loadNet(defPath)
		if err != nil {
			return nil, err
		}
		nets = append(nets, *n)
	}

	return nets, nil
}

func (e *podEnv) podNSPath() string {
	return filepath.Join(e.podRoot, "netns")
}

func (e *podEnv) netDir() string {
	return filepath.Join(e.podRoot, "net")
}

func (e *podEnv) setupNets(nets []activeNet) error {
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

	n := activeNet{}
	for i, n = range nets {
		log.Printf("Setup: executing net-plugin %v", n.conf.Type)

		n.runtime.IfName = fmt.Sprintf(ifnamePattern, i)
		if n.runtime.ConfPath, err = copyFileToDir(n.runtime.ConfPath, e.netDir()); err != nil {
			return fmt.Errorf("error copying %q to %q: %v", n.runtime.ConfPath, e.netDir(), err)
		}

		n.runtime.IP, n.hostIP, err = e.netPluginAdd(&n, nspath)
		if err != nil {
			return fmt.Errorf("error adding network %q: %v", n.conf.Name, err)
		}
	}

	return nil
}

func (e *podEnv) teardownNets(nets []activeNet) {
	nspath := e.podNSPath()

	for i := len(nets) - 1; i >= 0; i-- {
		log.Printf("Teardown: executing net-plugin %v", nets[i].conf.Type)

		err := e.netPluginDel(&nets[i], nspath)
		if err != nil {
			log.Printf("Error deleting %q: %v", nets[i].conf.Name, err)
		}

		// Delete the conf file to signal that the network was
		// torn down (or at least attempted to)
		if err = os.Remove(nets[i].runtime.ConfPath); err != nil {
			log.Printf("Error deleting %q: %v", nets[i].runtime.ConfPath, err)
		}
	}
}

func listFiles(dir string) ([]string, error) {
	dirents, err := ioutil.ReadDir(dir)
	switch {
	case err == nil:
	case os.IsNotExist(err):
		return nil, nil
	default:
		return nil, err
	}

	files := []string{}
	for _, dent := range dirents {
		if dent.IsDir() {
			continue
		}

		files = append(files, dent.Name())
	}

	return files, nil
}

func netExists(nets []activeNet, name string) bool {
	for _, n := range nets {
		if n.conf.Name == name {
			return true
		}
	}
	return false
}

func loadNet(filepath string) (*activeNet, error) {
	bytes, err := ioutil.ReadFile(filepath)
	if err != nil {
		return nil, err
	}

	n := &plugin.NetConf{}
	if err = json.Unmarshal(bytes, n); err != nil {
		return nil, fmt.Errorf("error loading %v: %v", filepath, err)
	}

	return &activeNet{
		confBytes: bytes,
		conf:      n,
		runtime: &netinfo.NetInfo{
			NetName:  n.Name,
			ConfPath: filepath,
		},
	}, nil
}

func copyFileToDir(src, dstdir string) (string, error) {
	dst := filepath.Join(dstdir, filepath.Base(src))

	s, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer s.Close()

	d, err := os.Create(dst)
	if err != nil {
		return "", err
	}
	defer d.Close()

	_, err = io.Copy(d, s)
	return dst, err
}

func loadUserNets(netsLoadList map[string]bool) ([]activeNet, error) {
	files, err := listFiles(UserNetPath)
	if err != nil {
		return nil, err
	}

	sort.Strings(files)

	nets := make([]activeNet, 0, len(files))

	for _, filename := range files {
		filepath := path.Join(UserNetPath, filename)

		if !strings.HasSuffix(filepath, ".conf") {
			continue
		}

		n, err := loadNet(filepath)
		if err != nil {
			return nil, err
		}

		if !(netsLoadList["true"] || netsLoadList[n.conf.Name]) {
			continue
		}

		// "default" is slightly special
		if n.conf.Name == "default" {
			log.Printf(`Overriding "default" network with %v`, filename)
		}

		if netExists(nets, n.conf.Name) {
			log.Printf("%q network already defined, ignoring %v", n.conf.Name, filename)
			continue
		}

		nets = append(nets, *n)
	}

	return nets, nil
}
