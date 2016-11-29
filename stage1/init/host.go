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
	"io"
	"os"
	"path/filepath"

	"github.com/appc/goaci/proj2aci"
	"github.com/coreos/rkt/common"
	"github.com/godbus/dbus"
	"github.com/godbus/dbus/introspect"
)

// Path to the localtime file/symlink in host
const localtimePath = "/etc/localtime"

// mirrorLocalZoneInfo tries to reproduce the /etc/localtime target in stage1/ to satisfy systemd-nspawn
func mirrorLocalZoneInfo(root string) {
	zif, err := os.Readlink(localtimePath)
	if err != nil {
		return
	}

	// On some systems /etc/localtime is a relative symlink, make it absolute
	if !filepath.IsAbs(zif) {
		zif = filepath.Join(filepath.Dir(localtimePath), zif)
		zif = filepath.Clean(zif)
	}

	src, err := os.Open(zif)
	if err != nil {
		return
	}
	defer src.Close()

	destp := filepath.Join(common.Stage1RootfsPath(root), zif)

	if err = os.MkdirAll(filepath.Dir(destp), 0755); err != nil {
		return
	}

	dest, err := os.OpenFile(destp, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer dest.Close()

	_, _ = io.Copy(dest, src)
}

// installAssets is used in the `host` flavor to copy host binaries
// in to the stage1 fs
func installAssets() error {
	systemctlBin, err := common.LookupPath("systemctl", os.Getenv("PATH"))
	if err != nil {
		return err
	}
	systemdSysusersBin, err := common.LookupPath("systemd-sysusers", os.Getenv("PATH"))
	if err != nil {
		return err
	}
	bashBin, err := common.LookupPath("bash", os.Getenv("PATH"))
	if err != nil {
		return err
	}
	mountBin, err := common.LookupPath("mount", os.Getenv("PATH"))
	if err != nil {
		return err
	}
	umountBin, err := common.LookupPath("umount", os.Getenv("PATH"))
	if err != nil {
		return err
	}
	// More paths could be added in that list if some Linux distributions install it in a different path
	// Note that we look in /usr/lib/... first because of the merge:
	// http://www.freedesktop.org/wiki/Software/systemd/TheCaseForTheUsrMerge/
	systemdShutdownBin, err := common.LookupPath("systemd-shutdown", "/usr/lib/systemd:/lib/systemd")
	if err != nil {
		return err
	}
	systemdBin, err := common.LookupPath("systemd", "/usr/lib/systemd:/lib/systemd")
	if err != nil {
		return err
	}
	systemdJournaldBin, err := common.LookupPath("systemd-journald", "/usr/lib/systemd:/lib/systemd")
	if err != nil {
		return err
	}

	systemdUnitsPath := "/lib/systemd/system"
	assets := []string{
		proj2aci.GetAssetString("/usr/lib/systemd/systemd", systemdBin),
		proj2aci.GetAssetString("/usr/bin/systemctl", systemctlBin),
		proj2aci.GetAssetString("/usr/bin/systemd-sysusers", systemdSysusersBin),
		proj2aci.GetAssetString("/usr/lib/systemd/systemd-journald", systemdJournaldBin),
		proj2aci.GetAssetString("/usr/bin/bash", bashBin),
		proj2aci.GetAssetString("/bin/mount", mountBin),
		proj2aci.GetAssetString("/bin/umount", umountBin),
		proj2aci.GetAssetString(fmt.Sprintf("%s/systemd-journald.service", systemdUnitsPath), fmt.Sprintf("%s/systemd-journald.service", systemdUnitsPath)),
		proj2aci.GetAssetString(fmt.Sprintf("%s/systemd-journald.socket", systemdUnitsPath), fmt.Sprintf("%s/systemd-journald.socket", systemdUnitsPath)),
		proj2aci.GetAssetString(fmt.Sprintf("%s/systemd-journald-dev-log.socket", systemdUnitsPath), fmt.Sprintf("%s/systemd-journald-dev-log.socket", systemdUnitsPath)),
		proj2aci.GetAssetString(fmt.Sprintf("%s/systemd-journald-audit.socket", systemdUnitsPath), fmt.Sprintf("%s/systemd-journald-audit.socket", systemdUnitsPath)),
		// systemd-shutdown has to be installed at the same path as on the host
		// because it depends on systemd build flag -DSYSTEMD_SHUTDOWN_BINARY_PATH=
		proj2aci.GetAssetString(systemdShutdownBin, systemdShutdownBin),
	}

	return proj2aci.PrepareAssets(assets, "./stage1/rootfs/", nil)
}

// hasMachinedRegister checks if nspawn should register the pod to machined
func hasMachinedRegister() bool {
	// machined has a D-Bus interface following versioning guidelines, see:
	// http://www.freedesktop.org/wiki/Software/systemd/machined/
	// Therefore we can just check if the D-Bus method we need exists and we
	// don't need to check the signature.
	var found int

	conn, err := dbus.SystemBus()
	if err != nil {
		return false
	}
	node, err := introspect.Call(conn.Object("org.freedesktop.machine1", "/org/freedesktop/machine1"))
	if err != nil {
		return false
	}
	for _, iface := range node.Interfaces {
		if iface.Name != "org.freedesktop.machine1.Manager" {
			continue
		}
		// machined v215 supports methods "RegisterMachine" and "CreateMachine" called by nspawn v215.
		// machined v216+ (since commit 5aa4bb) additionally supports methods "CreateMachineWithNetwork"
		// and "RegisterMachineWithNetwork", called by nspawn v216+.
		for _, method := range iface.Methods {
			if method.Name == "CreateMachineWithNetwork" || method.Name == "RegisterMachineWithNetwork" {
				found++
			}
		}
		break
	}
	return found == 2
}
