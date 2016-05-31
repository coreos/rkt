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

//+build linux

package cgroup

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/coreos/go-systemd/unit"
	"github.com/hashicorp/errwrap"
	"k8s.io/kubernetes/pkg/api/resource"
)

type addIsolatorFunc func(opts []*unit.UnitOption, limit *resource.Quantity) ([]*unit.UnitOption, error)

const (
	// The following const comes from
	// #define CGROUP2_SUPER_MAGIC  0x63677270
	// https://github.com/torvalds/linux/blob/v4.6/include/uapi/linux/magic.h#L58
	Cgroup2fsMagicNumber = 0x63677270
)

var (
	isolatorFuncs = map[string]addIsolatorFunc{
		"cpu":    addCpuLimit,
		"memory": addMemoryLimit,
	}
	legacyCgroupControllerRWFiles = map[string][]string{
		"memory": {"memory.limit_in_bytes"},
		"cpu":    {"cpu.cfs_quota_us"},
	}
	cgroupControllerRWFiles = map[string][]string{
		"memory": {"memory.low", "memory.high", "memory.max", "memory.swap.max"},
	}
)

func addCpuLimit(opts []*unit.UnitOption, limit *resource.Quantity) ([]*unit.UnitOption, error) {
	if limit.Value() > resource.MaxMilliValue {
		return nil, fmt.Errorf("cpu limit exceeds the maximum millivalue: %v", limit.String())
	}
	quota := strconv.Itoa(int(limit.MilliValue()/10)) + "%"
	opts = append(opts, unit.NewUnitOption("Service", "CPUQuota", quota))
	return opts, nil
}

func addMemoryLimit(opts []*unit.UnitOption, limit *resource.Quantity) ([]*unit.UnitOption, error) {
	opts = append(opts, unit.NewUnitOption("Service", "MemoryLimit", strconv.Itoa(int(limit.Value()))))
	return opts, nil
}

// MaybeAddIsolator considers the given isolator; if the type is known
// (i.e. IsIsolatorSupported is true) and the limit is non-nil, the supplied
// opts will be extended with an appropriate option implementing the desired
// isolation.
func MaybeAddIsolator(opts []*unit.UnitOption, isolator string, limit *resource.Quantity) ([]*unit.UnitOption, error) {
	var err error
	if limit == nil {
		return opts, nil
	}
	isSupported, err := IsIsolatorSupported(isolator)
	if err != nil {
		return nil, err
	}

	if isSupported {
		opts, err = isolatorFuncs[isolator](opts, limit)
	} else {
		fmt.Fprintf(os.Stderr, "warning: resource/%s isolator set but support disabled in the kernel, skipping\n", isolator)
	}

	if err != nil {
		return nil, err
	}
	return opts, nil
}

// IsIsolatorSupported returns whether an isolator is supported in the kernel
func IsIsolatorSupported(isolator string) (bool, error) {
	procCgroupFile, err := os.Open("/proc/cgroups")
	if err != nil {
		return false, err
	}
	defer procCgroupFile.Close()

	sc := bufio.NewScanner(procCgroupFile)

	sc.Scan()

	for sc.Scan() {
		if strings.HasPrefix(sc.Text(), isolator) {
			return true, nil
		}
	}

	if err := sc.Err(); err != nil {
		return false, err
	}

	return false, nil
}

// IsCgroupUnified checks if cgroup mounted at /sys/fs/cgroup is
// the new unified version (cgroup v2)
func IsCgroupUnified(root string) (bool, error) {
	cgroupFsPath := filepath.Join(root, "/sys/fs/cgroup")
	var statfs syscall.Statfs_t
	if err := syscall.Statfs(cgroupFsPath, &statfs); err != nil {
		return false, err
	}

	return statfs.Type == Cgroup2fsMagicNumber, nil
}

func parseLegacyCgroups(f io.Reader) (map[int][]string, error) {
	sc := bufio.NewScanner(f)

	// skip first line since it is a comment
	sc.Scan()

	cgroups := make(map[int][]string)
	for sc.Scan() {
		var controller string
		var hierarchy int
		var num int
		var enabled int
		fmt.Sscanf(sc.Text(), "%s %d %d %d", &controller, &hierarchy, &num, &enabled)

		if enabled == 1 {
			if _, ok := cgroups[hierarchy]; !ok {
				cgroups[hierarchy] = []string{controller}
			} else {
				cgroups[hierarchy] = append(cgroups[hierarchy], controller)
			}
		}
	}

	if err := sc.Err(); err != nil {
		return nil, err
	}

	return cgroups, nil
}

// GetEnabledLegacyCgroups returns a map with the enabled cgroup controllers grouped by
// hierarchy
func GetEnabledLegacyCgroups() (map[int][]string, error) {
	cgroupsFile, err := os.Open("/proc/cgroups")
	if err != nil {
		return nil, err
	}
	defer cgroupsFile.Close()

	cgroups, err := parseLegacyCgroups(cgroupsFile)
	if err != nil {
		return nil, errwrap.Wrap(errors.New("error parsing /proc/cgroups"), err)
	}

	return cgroups, nil
}

// GetEnabledControllers raturns a list of enabled cgroup controllers
func GetEnabledControllers() ([]string, error) {
	controllersFile, err := os.Open("/sys/fs/cgroup/cgroup.controllers")
	if err != nil {
		return nil, err
	}
	defer controllersFile.Close()

	sc := bufio.NewScanner(controllersFile)

	sc.Scan()
	if err := sc.Err(); err != nil {
		return nil, err
	}

	return strings.Split(sc.Text(), " "), nil
}

// GetLegacyControllerDirs takes a map with the enabled cgroup controllers grouped by
// hierarchy and returns the directory names as they should be in
// /sys/fs/cgroup
func GetLegacyControllerDirs(cgroups map[int][]string) []string {
	var controllers []string
	for _, cs := range cgroups {
		controllers = append(controllers, strings.Join(cs, ","))
	}

	return controllers
}

func getLegacyControllerSymlinks(cgroups map[int][]string) map[string]string {
	symlinks := make(map[string]string)

	for _, cs := range cgroups {
		if len(cs) > 1 {
			tgt := strings.Join(cs, ",")
			for _, ln := range cs {
				symlinks[ln] = tgt
			}
		}
	}

	return symlinks
}

func getLegacyControllerRWFiles(controller string) []string {
	parts := strings.Split(controller, ",")
	for _, p := range parts {
		if files, ok := legacyCgroupControllerRWFiles[p]; ok {
			// cgroup.procs always needs to be RW for allowing systemd to add
			// processes to the controller
			files = append(files, "cgroup.procs")
			return files
		}
	}

	return nil
}

func getControllersRWFiles() []string {
	files := []string{"cgroup.procs", "cgroup.controllers", "cgroup.subtree_control", "cgroup.events"}
	for _, controllerRWFiles := range cgroupControllerRWFiles {
		files = append(files, controllerRWFiles...)
	}

	return files
}

func parseLegacyCgroupController(cgroupPath, controller string) ([]string, error) {
	cg, err := os.Open(cgroupPath)
	if err != nil {
		return nil, errwrap.Wrap(errors.New("error opening /proc/self/cgroup"), err)
	}
	defer cg.Close()

	s := bufio.NewScanner(cg)
	for s.Scan() {
		parts := strings.SplitN(s.Text(), ":", 3)
		if len(parts) < 3 {
			return nil, fmt.Errorf("error parsing /proc/self/cgroup")
		}
		controllerParts := strings.Split(parts[1], ",")
		for _, c := range controllerParts {
			if c == controller {
				return parts, nil
			}
		}
	}

	return nil, fmt.Errorf("controller %q not found", controller)
}

func parseProcCgroupInfo(procCgroupInfoPath string) (string, error) {
	cg, err := os.Open(procCgroupInfoPath)
	if err != nil {
		return "", errwrap.Wrap(errors.New("error opening /proc/self/cgroup"), err)
	}
	defer cg.Close()

	s := bufio.NewScanner(cg)
	s.Scan()
	parts := strings.SplitN(s.Text(), ":", 3)
	if len(parts) < 3 {
		return "", fmt.Errorf("error parsing /proc/self/cgroup")
	}

	return parts[2], nil
}

// GetOwnLegacyCgroupPath returns the cgroup path of this process in controller
// hierarchy
func GetOwnLegacyCgroupPath(controller string) (string, error) {
	parts, err := parseLegacyCgroupController("/proc/self/cgroup", controller)
	if err != nil {
		return "", err
	}
	return parts[2], nil
}

// GetOwnCgroupPath returns the cgroup path of this process
func GetOwnCgroupPath() (string, error) {
	return parseProcCgroupInfo("/proc/self/cgroup")
}

// GetLegacyCgroupPathByPid returns the cgroup path of the process with the given pid
// and given controller.
func GetLegacyCgroupPathByPid(pid int, controller string) (string, error) {
	parts, err := parseLegacyCgroupController(fmt.Sprintf("/proc/%d/cgroup", pid), controller)
	if err != nil {
		return "", err
	}
	return parts[2], nil
}

// GetCgroupPathByPid returns the cgroup path of the process
func GetCgroupPathByPid(pid int) (string, error) {
	return parseProcCgroupInfo(fmt.Sprintf("/proc/%d/cgroup", pid))
}

// JoinCgroup makes the calling process join the subcgroup hierarchy on a
// particular controller
func JoinLegacySubcgroup(controller string, subcgroup string) error {
	subcgroupPath := filepath.Join("/sys/fs/cgroup", controller, subcgroup)
	if err := os.MkdirAll(subcgroupPath, 0600); err != nil {
		return errwrap.Wrap(fmt.Errorf("error creating %q subcgroup", subcgroup), err)
	}
	pidBytes := []byte(strconv.Itoa(os.Getpid()))
	if err := ioutil.WriteFile(filepath.Join(subcgroupPath, "cgroup.procs"), pidBytes, 0600); err != nil {
		return errwrap.Wrap(fmt.Errorf("error adding ourselves to the %q subcgroup", subcgroup), err)
	}

	return nil
}

// JoinCgroup makes the calling process join the subcgroup hierarchy
func JoinSubcgroup(subcgroup string) error {
	subcgroupPath := filepath.Join("/sys/fs/cgroup", subcgroup)
	if err := os.MkdirAll(subcgroupPath, 0600); err != nil {
		return errwrap.Wrap(fmt.Errorf("error creating %q subcgroup", subcgroup), err)
	}
	pidBytes := []byte(strconv.Itoa(os.Getpid()))
	if err := ioutil.WriteFile(filepath.Join(subcgroupPath, "cgroup.procs"), pidBytes, 0600); err != nil {
		return errwrap.Wrap(fmt.Errorf("error adding ourselves to the %q subcgroup", subcgroup), err)
	}

	return nil
}

// If /system.slice does not exist in the cpuset controller, create it and
// configure it.
// Since this is a workaround, we ignore errors
func fixCpusetKnobs(cpusetPath string) {
	cgroupPathFix := filepath.Join(cpusetPath, "system.slice")
	_ = os.MkdirAll(cgroupPathFix, 0755)
	knobs := []string{"cpuset.mems", "cpuset.cpus"}
	for _, knob := range knobs {
		parentFile := filepath.Join(filepath.Dir(cgroupPathFix), knob)
		childFile := filepath.Join(cgroupPathFix, knob)

		data, err := ioutil.ReadFile(childFile)
		if err != nil {
			continue
		}
		// If the file is already configured, don't change it
		if strings.TrimSpace(string(data)) != "" {
			continue
		}

		data, err = ioutil.ReadFile(parentFile)
		if err == nil {
			// Workaround: just write twice to workaround the kernel bug fixed by this commit:
			// https://github.com/torvalds/linux/commit/24ee3cf89bef04e8bc23788aca4e029a3f0f06d9
			ioutil.WriteFile(childFile, data, 0644)
			ioutil.WriteFile(childFile, data, 0644)
		}
	}
}

// IsLegacyControllerMounted returns whether a controller is mounted by checking that
// cgroup.procs is accessible
func IsLegacyControllerMounted(c string) bool {
	cgroupProcsPath := filepath.Join("/sys/fs/cgroup", c, "cgroup.procs")
	if _, err := os.Stat(cgroupProcsPath); err != nil {
		return false
	}

	return true
}

// CreateLegacyCgroups mounts the cgroup controllers hierarchy in /sys/fs/cgroup
// under root
func CreateLegacyCgroups(root string, enabledCgroups map[int][]string, mountContext string) error {
	controllers := GetLegacyControllerDirs(enabledCgroups)
	var flags uintptr

	sys := filepath.Join(root, "/sys")
	if err := os.MkdirAll(sys, 0700); err != nil {
		return err
	}
	flags = syscall.MS_NOSUID |
		syscall.MS_NOEXEC |
		syscall.MS_NODEV
	// If we're mounting the host cgroups, /sys is probably mounted so we
	// ignore EBUSY
	if err := syscall.Mount("sysfs", sys, "sysfs", flags, ""); err != nil && err != syscall.EBUSY {
		return errwrap.Wrap(fmt.Errorf("error mounting %q", sys), err)
	}

	cgroupTmpfs := filepath.Join(root, "/sys/fs/cgroup")
	if err := os.MkdirAll(cgroupTmpfs, 0700); err != nil {
		return err
	}
	flags = syscall.MS_NOSUID |
		syscall.MS_NOEXEC |
		syscall.MS_NODEV |
		syscall.MS_STRICTATIME

	options := "mode=755"
	if mountContext != "" {
		options = fmt.Sprintf("mode=755,context=\"%s\"", mountContext)
	}

	if err := syscall.Mount("tmpfs", cgroupTmpfs, "tmpfs", flags, options); err != nil {
		return errwrap.Wrap(fmt.Errorf("error mounting %q", cgroupTmpfs), err)
	}

	// Mount controllers
	for _, c := range controllers {
		cPath := filepath.Join(root, "/sys/fs/cgroup", c)
		if err := os.MkdirAll(cPath, 0700); err != nil {
			return err
		}

		flags = syscall.MS_NOSUID |
			syscall.MS_NOEXEC |
			syscall.MS_NODEV
		if err := syscall.Mount("cgroup", cPath, "cgroup", flags, c); err != nil {
			return errwrap.Wrap(fmt.Errorf("error mounting %q", cPath), err)
		}
	}

	// Create symlinks for combined controllers
	symlinks := getLegacyControllerSymlinks(enabledCgroups)
	for ln, tgt := range symlinks {
		lnPath := filepath.Join(cgroupTmpfs, ln)
		if err := os.Symlink(tgt, lnPath); err != nil {
			return errwrap.Wrap(errors.New("error creating symlink"), err)
		}
	}

	systemdControllerPath := filepath.Join(root, "/sys/fs/cgroup/systemd")
	if err := os.MkdirAll(systemdControllerPath, 0700); err != nil {
		return err
	}

	// Bind-mount cgroup tmpfs filesystem read-only
	flags = syscall.MS_BIND |
		syscall.MS_REMOUNT |
		syscall.MS_NOSUID |
		syscall.MS_NOEXEC |
		syscall.MS_NODEV |
		syscall.MS_RDONLY
	if err := syscall.Mount(cgroupTmpfs, cgroupTmpfs, "", flags, ""); err != nil {
		return errwrap.Wrap(fmt.Errorf("error remounting RO %q", cgroupTmpfs), err)
	}

	return nil
}

// CreateCgroups mounts the unigfied cgroup hierarchy in /sys/fs/cgroup
// under root
func CreateCgroups(root string) error {
	var flags uintptr

	sys := filepath.Join(root, "/sys")
	if err := os.MkdirAll(sys, 0700); err != nil {
		return err
	}
	flags = syscall.MS_NOSUID |
		syscall.MS_NOEXEC |
		syscall.MS_NODEV

	if err := syscall.Mount("sysfs", sys, "sysfs", flags, ""); err != nil {
		return errwrap.Wrap(fmt.Errorf("error mounting %q", sys), err)
	}

	cgroupFs := filepath.Join(root, "/sys/fs/cgroup")
	if err := os.MkdirAll(cgroupFs, 0700); err != nil {
		return err
	}

	flags = syscall.MS_NOSUID |
		syscall.MS_NOEXEC |
		syscall.MS_NODEV |
		syscall.MS_STRICTATIME

	if err := syscall.Mount("cgroup2", cgroupFs, "cgroup2", flags, ""); err != nil {
		return errwrap.Wrap(fmt.Errorf("error mounting %q", cgroupFs), err)
	}

	machinePath := filepath.Join(root, "/sys/fs/cgroup/machine.slice")
	if err := os.MkdirAll(machinePath, 0700); err != nil {
		return err
	}

	return nil
}

// RemountCgroupsRO remounts the cgroup hierarchy under root read-only, leaving
// the needed knobs in the subcgroup for each app read-write so the systemd
// inside stage1 can apply isolators to them
func RemountLegacyCgroupsRO(root string, enabledCgroups map[int][]string, subcgroup string, serviceNames []string) error {
	controllers := GetLegacyControllerDirs(enabledCgroups)
	cgroupTmpfs := filepath.Join(root, "/sys/fs/cgroup")
	sysPath := filepath.Join(root, "/sys")

	var flags uintptr

	// Mount RW knobs we need to make the enabled isolators work
	for _, c := range controllers {
		cPath := filepath.Join(cgroupTmpfs, c)
		subcgroupPath := filepath.Join(cPath, subcgroup, "system.slice")

		// Workaround for https://github.com/coreos/rkt/issues/1210
		if c == "cpuset" {
			fixCpusetKnobs(cPath)
		}

		// Create cgroup directories and mount the files we need over
		// themselves so they stay read-write
		for _, serviceName := range serviceNames {
			appCgroup := filepath.Join(subcgroupPath, serviceName)
			if err := os.MkdirAll(appCgroup, 0755); err != nil {
				return err
			}
			for _, f := range getLegacyControllerRWFiles(c) {
				cgroupFilePath := filepath.Join(appCgroup, f)
				// the file may not be there if kernel doesn't support the
				// feature, skip it in that case
				if _, err := os.Stat(cgroupFilePath); os.IsNotExist(err) {
					continue
				}
				if err := syscall.Mount(cgroupFilePath, cgroupFilePath, "", syscall.MS_BIND, ""); err != nil {
					return errwrap.Wrap(fmt.Errorf("error bind mounting %q", cgroupFilePath), err)
				}
			}
		}

		// Re-mount controller read-only to prevent the container modifying host controllers
		flags = syscall.MS_BIND |
			syscall.MS_REMOUNT |
			syscall.MS_NOSUID |
			syscall.MS_NOEXEC |
			syscall.MS_NODEV |
			syscall.MS_RDONLY
		if err := syscall.Mount(cPath, cPath, "", flags, ""); err != nil {
			return errwrap.Wrap(fmt.Errorf("error remounting RO %q", cPath), err)
		}
	}

	// Bind-mount sys filesystem read-only
	flags = syscall.MS_BIND |
		syscall.MS_REMOUNT |
		syscall.MS_NOSUID |
		syscall.MS_NOEXEC |
		syscall.MS_NODEV |
		syscall.MS_RDONLY
	if err := syscall.Mount(sysPath, sysPath, "", flags, ""); err != nil {
		return errwrap.Wrap(fmt.Errorf("error remounting RO %q", sysPath), err)
	}

	return nil
}

// RemountCgroupsRO remounts the cgroup hierarchy under root read-only, leaving
// the needed knobs in the subcgroup for each app read-write so the systemd
// inside stage1 can apply isolators to them
func RemountCgroupsRO(root string, subcgroup string, serviceNames []string) error {
	cgroupFsPath := filepath.Join(root, "sys/fs/cgroup")
	subcgroupPath := filepath.Join(root, "sys/fs/cgroup", subcgroup)
	sysPath := filepath.Join(root, "/sys")

	var flags uintptr

	if err := os.MkdirAll(subcgroupPath, 0700); err != nil {
		return err
	}

	machineDir := strings.TrimSuffix(subcgroupPath, "/system.slice")
	if err := syscall.Mount(machineDir, machineDir, "", syscall.MS_BIND, ""); err != nil {
		return errwrap.Wrap(fmt.Errorf("error bind mounting %q", subcgroupPath), err)
	}

	// Bind-mount cgroup filesystem read-only
	flags = syscall.MS_BIND |
		syscall.MS_REMOUNT |
		syscall.MS_NOSUID |
		syscall.MS_NOEXEC |
		syscall.MS_NODEV |
		syscall.MS_RDONLY
	if err := syscall.Mount(cgroupFsPath, cgroupFsPath, "", flags, ""); err != nil {
		return errwrap.Wrap(fmt.Errorf("error remounting RO %q", cgroupFsPath), err)
	}

	// Bind-mount sys filesystem read-only
	flags = syscall.MS_BIND |
		syscall.MS_REMOUNT |
		syscall.MS_NOSUID |
		syscall.MS_NOEXEC |
		syscall.MS_NODEV |
		syscall.MS_RDONLY
	if err := syscall.Mount(sysPath, sysPath, "", flags, ""); err != nil {
		return errwrap.Wrap(fmt.Errorf("error remounting RO %q", sysPath), err)
	}

	return nil
}
