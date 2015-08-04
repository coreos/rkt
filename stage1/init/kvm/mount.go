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

// mount.go provides functions for creating a mount units for
// managing inner(kind=empty) and external(kind=host) volumes
// important: used only for kvm flavor (lkvm based)
// idea:
// for example when we have two volumes:
// --volume=hostdata,kind=host,source=/host/some_data_to_share
// --volume=temporary,kind=empty
// in stage1/rootfs rkt creates two folders (in rootfs of guest)
// 1. /mnt/hostdata - which is mounted through 9p host thanks to lkvm --9p=/host/some_data_to_share,hostdata flag shared to quest
// 2. /mnt/temporary - is created as empty directory in guest
// both of them are then bind mounted to /opt/stage2/<application/<mountPoint.path> for every application, that has mountPoints specified in ACI json
// - host mounting is realized by podToSystemdHostMountUnits (for whole pod)  which creates mount.units (9p) required and ordered before all applications service units
// - bind mounting is realized by appToSystemdMountUnits (for each app) which creates mount.units (bind) required and ordered before particular application
// note: systemd mount units require /usr/bin/mount
package kvm

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/schema/types"
	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/coreos/go-systemd/unit"
	"github.com/coreos/rkt/common"
)

const (
	// location within stage1 rootfs where shared volumes will be put
	// (or empty directories for kind=empty)
	stage1MntDir = "/mnt/"
)

// serviceUnitName returns a systemd service unit name for the given app name.
// note: it was shamefully copy-pasted from stage1/init/path.go
// TODO: extract common functions from path.go
func serviceUnitName(appName types.ACName) string {
	return appName.String() + ".service"
}

// installNewMountUnit utility function creates and installs new mount unit
// in default systemd location (/usr/lib/systemd/system) in pod stage1 filesystem.
// root is a stage1 relative pod path filesystem like /var/lib/uuid/rootfs/ (from Pod.Root)
// beforeAndrequiredBy creates systemd unit dependency (can be space separated for multi)
func installNewMountUnit(root, what, where, fsType, options, beforeAndrequiredBy, unitsDir string) error {

	opts := []*unit.UnitOption{
		unit.NewUnitOption("Unit", "Description", fmt.Sprintf("Mount unit for %s", where)),
		unit.NewUnitOption("Unit", "DefaultDependencies", "false"),
		unit.NewUnitOption("Unit", "Before", beforeAndrequiredBy),
		unit.NewUnitOption("Mount", "What", what),
		unit.NewUnitOption("Mount", "Where", where),
		unit.NewUnitOption("Mount", "Type", fsType),
		unit.NewUnitOption("Mount", "Options", options),
		unit.NewUnitOption("Install", "RequiredBy", beforeAndrequiredBy),
	}

	unitsPath := filepath.Join(root, unitsDir)
	unitName := unit.UnitNamePathEscape(where + ".mount")
	unitBytes, err := ioutil.ReadAll(unit.Serialize(opts))
	if err != nil {
		return fmt.Errorf("failed to serialize mount unit file to bytes %q: %v", unitName, err)
	}

	err = ioutil.WriteFile(filepath.Join(unitsPath, unitName), unitBytes, 0644)
	if err != nil {
		return fmt.Errorf("failed to create mount unit file %q: %v", unitName, err)
	}

	log.Printf("mount unit created: %q in %q (what=%q, where=%q)", unitName, unitsPath, what, where)
	return nil
}

// PodToSystemdHostMountUnits create host shared remote file system mounts (using e.g. 9p)
// according to https://www.kernel.org/doc/Documentation/filesystems/9p.txt.
// Additionally create required directories in stage1MntDir and then prepares bind mount unit
// for each app.
// 'root' parameter is stage1 root filesystem path.
// appNames are used to create before/required dependency between mount unit and app service units
func PodToSystemdHostMountUnits(root string, volumes []types.Volume, appNames []types.ACName, unitsDir string) error {

	// pod volumes need to mount p9 qemu mount_tags
	for _, vol := range volumes {
		// only host shared volumes

		name := vol.Name.String() // acts as a mount tag 9p
		// /var/lib/.../pod/run/rootfs/mnt/{volumeName}
		mountPoint := filepath.Join(root, stage1MntDir, name)

		// for kind "empty" that will be shared among applications
		log.Printf("creating an empty volume folder for sharing: %q", mountPoint)
		err := os.MkdirAll(mountPoint, 0700)
		if err != nil {
			return err
		}

		// serviceNames for ordering and requirements dependency for apps
		serviceNames := []string{}
		for _, appName := range appNames {
			serviceNames = append(serviceNames, serviceUnitName(appName))
		}

		// for host kind we create a mount unit to mount host shared folder
		if vol.Kind == "host" {
			err = installNewMountUnit(root,
				name, // what (source) in 9p it is a channel tag which equals to volume.Name/mountPoint.name
				filepath.Join(stage1MntDir, name), // where - destination
				"9p",                            // fsType
				"trans=virtio",                  // 9p specific options
				strings.Join(serviceNames, " "), // space separated list of services for unit dependency
				unitsDir,
			)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// AppToSystemdMountUnits prepare bind mount unit for empty or host kind mounting
// between stage1 rootfs and chrooted filesystem for application
func AppToSystemdMountUnits(root string, appName types.ACName, mountPoints []types.MountPoint, unitsDir string) error {

	for _, mountPoint := range mountPoints {

		name := mountPoint.Name.String()
		// source relative to stage1 rootfs to relative pod root
		whatPath := filepath.Join(stage1MntDir, name)
		whatFullPath := filepath.Join(root, whatPath)

		// destination relative to stage1 rootfs and relative to pod root
		wherePath := filepath.Join(common.RelAppRootfsPath(appName), mountPoint.Path)
		whereFullPath := filepath.Join(root, wherePath)

		// readOnly
		mountOptions := "bind"
		if mountPoint.ReadOnly {
			mountOptions += ",ro"
		}

		// assertion to make sure that "what" exists (created earlier by podToSystemdHostMountUnits)
		log.Printf("checking required source path: %q", whatFullPath)
		if _, err := os.Stat(whatFullPath); os.IsNotExist(err) {
			return fmt.Errorf("app requires a volume that is not defined in Pod (try adding --volume=%s,kind=empty)!", name)
		}

		// optionally prepare app directory
		log.Printf("optionally preparing destination path: %q", whereFullPath)
		err := os.MkdirAll(whereFullPath, 0700)
		if err != nil {
			return fmt.Errorf("failed to prepare dir for mountPoint %v: %v", mountPoint.Name, err)
		}

		// install new mount unit for bind mount /mnt/volumenName -> /opt/stage2/{app-id}/rootfs/{{mountPoint.Path}}
		err = installNewMountUnit(
			root,      // where put a mount unit
			whatPath,  // what - stage1 rootfs /mnt/VolumeName
			wherePath, // where - inside chroot app filesystem
			"bind",    // fstype
			mountOptions,
			serviceUnitName(appName),
			unitsDir,
		)
		if err != nil {
			return fmt.Errorf("cannot install new mount unit error for app %q: %v", appName.String(), err)
		}

	}
	return nil
}

// VolumesToKvmDiskArgs renders a prepared Pod
// argument list ready to be passed to lkvm.
// Theses arguments allow exporting pod volumes of kind host to quest.
// eg. output is --9p,src/folder,9ptag
func VolumesToKvmDiskArgs(volumes []types.Volume) []string {
	args := []string{}

	for _, vol := range volumes {
		mountTag := vol.Name.String() // tag/channel name for virtio
		if vol.Kind == "host" {
			// eg. --9p=/home/jon/srcdir,tag
			arg := "--9p=" + vol.Source + "," + mountTag
			log.Printf("stage1: --disk argument: %#v\n", arg)
			args = append(args, arg)
		}
	}

	return args
}
