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
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"

	"github.com/hashicorp/errwrap"

	"github.com/containernetworking/cni/pkg/ns"
	cnitypes "github.com/containernetworking/cni/pkg/types"

	"github.com/rkt/rkt/common"
	"github.com/rkt/rkt/networking/netinfo"
)

const (
	// Suffix to LocalConfigDir path, where users place their net configs
	UserNetPathSuffix = "net.d"

	// Default net path relative to stage1 root
	DefaultNetPath           = "etc/rkt/net.d/99-default.conf"
	DefaultRestrictedNetPath = "etc/rkt/net.d/99-default-restricted.conf"
)

// Loads nets specified by user and default one from stage1
func (n *Networking) loadNets() ([]ActiveNet, error) {
	netList := n.Pod.NetList
	nets, err := loadUserNets(n.LocalConfigDir, netList)
	if err != nil {
		return nil, err
	}

	if netList.None() {
		return nets, nil
	}

	if !netExists(nets, "default") && !netExists(nets, "default-restricted") {
		var defaultNet string
		if netList.Specific("default") || netList.All() {
			defaultNet = DefaultNetPath
		} else {
			defaultNet = DefaultRestrictedNetPath
		}
		defPath := path.Join(common.Stage1RootfsPath(n.Pod.Root), defaultNet)
		net, err := loadNet(defPath)
		if err != nil {
			return nil, err
		}
		nets = append(nets, *net)
	}

	missing := missingNets(netList, nets)
	if len(missing) > 0 {
		return nil, fmt.Errorf("networks not found: %v", strings.Join(missing, ", "))
	}

	// Add the runtime args to the network instances.
	// We don't do this earlier because we also load networks in other contexts
	for _, net := range nets {
		net.Runtime.Args = netList.SpecificArgs(net.Conf.Name)
	}
	return nets, nil
}

// podNSCreate creates the network namespace and saves a reference to its path.
// NewNS will bind-mount the namespace in /run/netns, so we write that filename
// to disk.
func (n *Networking) podNSCreate() error {
	podNS, err := ns.NewNS()
	if err != nil {
		return err
	}
	n.PodNS = podNS

	if err := n.podNSPathSave(); err != nil {
		return err
	}
	return nil
}

func podNSFilePath(podRoot string) string {
	return filepath.Join(podRoot, "netns")
}

// podNSPathLoad reads the file where we wrote the real path to the pod netns.
func podNSPathLoad(podRoot string) (string, error) {
	podNSPath, err := ioutil.ReadFile(podNSFilePath(podRoot))
	if err != nil {
		return "", err
	}

	return string(podNSPath), nil
}

func podNSerrorOK(podNSPath string, err error) bool {
	switch err.(type) {
	case ns.NSPathNotExistErr:
		return true
	case ns.NSPathNotNSErr:
		return true

	default:
		if os.IsNotExist(err) {
			return true
		}
		return false
	}
}

func podNSLoad(podRoot string) (ns.NetNS, error) {
	podNSPath, err := podNSPathLoad(podRoot)
	if err != nil && !podNSerrorOK(podNSPath, err) {
		return nil, err
	} else {
		podNS, err := ns.GetNS(podNSPath)
		if err != nil && !podNSerrorOK(podNSPath, err) {
			return nil, err
		}
		return podNS, nil
	}
}

// podNSPathSave writes the path to the pod netns to a file (as a string)
func (n *Networking) podNSPathSave() error {
	podNSFile, err := os.OpenFile(podNSFilePath(n.Pod.Root), os.O_WRONLY|os.O_CREATE, 0)
	if err != nil {
		return err
	}
	defer podNSFile.Close()

	if _, err = io.WriteString(podNSFile, n.PodNS.Path()); err != nil {
		return err
	}

	return nil
}

func podNSDestroy(netns ns.NetNS) error {
	if netns == nil {
		return nil
	}

	// Close the namespace handle
	// If this handle also *created* the namespace, it will delete it for us.
	_ = netns.Close()

	// We still need to try and delete the namespace ourselves - no way to know
	// if podNS.Close() did it for us.
	// Unmount the ns bind-mount, and delete the mountpoint if successful
	nsPath := netns.Path()

	if err := syscall.Unmount(nsPath, unix.MNT_DETACH); err != nil {
		// if already unmounted, umount(2) returns EINVAL - continue
		if !os.IsNotExist(err) && err != syscall.EINVAL {
			return errwrap.Wrap(fmt.Errorf("error unmounting netns %q", nsPath), err)
		}
	}
	if err := os.RemoveAll(nsPath); err != nil {
		if !os.IsNotExist(err) {
			return errwrap.Wrap(fmt.Errorf("failed to remove netns %s", nsPath), err)
		}
	}
	return nil
}

func (n *Networking) netDir() string {
	return filepath.Join(n.Pod.Root, "net")
}

func (n *Networking) setupNets() error {
	err := os.MkdirAll(n.netDir(), 0755)
	if err != nil {
		return err
	}

	i := 0
	defer func() {
		if err != nil && i != 0 {
			n.teardownNets(i - 1)
		}
	}()

	// did stage0 already make /etc/rkt-resolv.conf (i.e. --dns passed)
	resolvPath := filepath.Join(common.Stage1RootfsPath(n.Pod.Root), "etc/rkt-resolv.conf")
	_, err = os.Stat(resolvPath)
	if err != nil && !os.IsNotExist(err) {
		return errwrap.Wrap(fmt.Errorf("error statting /etc/rkt-resolv.conf"), err)
	}
	podHasResolvConf := err == nil

	for i, net := range n.nets {
		if debuglog {
			stderr.Printf("loading network %v with type %v", net.Conf.Name, net.Conf.Type)
		}

		net.Runtime.IfName = fmt.Sprintf(IfNamePattern, i)
		if net.Runtime.ConfPath, err = copyFileToDir(net.Runtime.ConfPath, n.netDir()); err != nil {
			return errwrap.Wrap(fmt.Errorf("error copying %q to %q", net.Runtime.ConfPath, n.netDir()), err)
		}

		// Actually shell out to the plugin
		if n.AddNet != nil {
			err = n.AddNet(n, &net)
		} else {
			err = n.netPluginAdd(&net)
		}
		if err != nil {
			return errwrap.Wrap(fmt.Errorf("error adding network %q", net.Conf.Name), err)
		}

		// Generate rkt-resolv.conf if it's not already there.
		// The first network plugin that supplies a non-empty
		// DNS response will win, unless noDNS is true (--dns passed to rkt run)
		if !common.IsDNSZero(&net.Runtime.DNS) && !n.NoDNS() {
			if !podHasResolvConf {
				err := ioutil.WriteFile(
					resolvPath,
					[]byte(common.MakeResolvConf(net.Runtime.DNS, "Generated by rkt from network "+net.Conf.Name)),
					0644)
				if err != nil {
					return errwrap.Wrap(fmt.Errorf("error creating resolv.conf"), err)
				}
				podHasResolvConf = true
			} else {
				stderr.Printf("Warning: network %v plugin specified DNS configuration, but DNS already supplied", net.Conf.Name)
			}
		}
	}
	return nil
}

// teardownNets will call the DEL on every network, up to limit (or all of them,
// in case it is -1). This is used if we have to tear down in a partial case.
func (n *Networking) teardownNets(limit int) {
	i := limit
	if i == -1 {
		i = len(n.nets) - 1
	}

	for ; i >= 0; i-- {
		if debuglog {
			stderr.Printf("teardown - executing net-plugin %v", n.nets[i].Conf.Type)
		}

		var err error
		if n.DelNet != nil {
			err = n.DelNet(n, &n.nets[i])
		} else {
			err = n.netPluginDel(&n.nets[i])
		}
		if err != nil {
			stderr.PrintE(fmt.Sprintf("error deleting %q", n.nets[i].Conf.Name), err)
		}

		// Delete the conf file to signal that the network was
		// torn down (or at least attempted to)
		if err = os.Remove(n.nets[i].Runtime.ConfPath); err != nil {
			stderr.PrintE(fmt.Sprintf("error deleting %q", n.nets[i].Runtime.ConfPath), err)
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

	var files []string
	for _, dent := range dirents {
		if dent.IsDir() {
			continue
		}

		files = append(files, dent.Name())
	}

	return files, nil
}

func netExists(nets []ActiveNet, name string) bool {
	for _, n := range nets {
		if n.Conf.Name == name {
			return true
		}
	}
	return false
}

func loadNet(filepath string) (*ActiveNet, error) {
	bytes, err := ioutil.ReadFile(filepath)
	if err != nil {
		return nil, err
	}

	n := &cnitypes.NetConf{}
	if err = json.Unmarshal(bytes, n); err != nil {
		return nil, errwrap.Wrap(fmt.Errorf("error loading %v", filepath), err)
	}

	return &ActiveNet{
		ConfBytes: bytes,
		Conf:      n,
		Runtime: &netinfo.NetInfo{
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

// loadUserNets will load all network configuration files from the user-supplied
// configuration directory (typically /etc/rkt/net.d). Do not do any mutation here -
// we also load networks in a few other code paths.
func loadUserNets(localConfig string, netsLoadList common.NetList) ([]ActiveNet, error) {
	if netsLoadList.None() {
		stderr.Printf("networking namespace with loopback only")
		return nil, nil
	}

	userNetPath := filepath.Join(localConfig, UserNetPathSuffix)
	if debuglog {
		stderr.Printf("loading networks from %v", userNetPath)
	}

	files, err := listFiles(userNetPath)
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	nets := make([]ActiveNet, 0, len(files))

	for _, filename := range files {
		filepath := filepath.Join(userNetPath, filename)

		if !strings.HasSuffix(filepath, ".conf") {
			continue
		}

		n, err := loadNet(filepath)
		if err != nil {
			return nil, err
		}

		if !(netsLoadList.All() || netsLoadList.Specific(n.Conf.Name)) {
			continue
		}

		if n.Conf.Name == "default" ||
			n.Conf.Name == "default-restricted" {
			stderr.Printf(`overriding %q network with %v`, n.Conf.Name, filename)
		}

		if netExists(nets, n.Conf.Name) {
			stderr.Printf("%q network already defined, ignoring %v", n.Conf.Name, filename)
			continue
		}

		nets = append(nets, *n)
	}

	return nets, nil
}

func missingNets(defined common.NetList, loaded []ActiveNet) []string {
	diff := make(map[string]struct{})
	for _, n := range defined.StringsOnlyNames() {
		if n != "all" {
			diff[n] = struct{}{}
		}
	}

	for _, an := range loaded {
		delete(diff, an.Conf.Name)
	}

	var missing []string
	for n := range diff {
		missing = append(missing, n)
	}
	return missing
}
