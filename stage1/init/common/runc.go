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

//+build runc

package common

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/appc/spec/schema/types"
	"github.com/coreos/go-systemd/unit"
	"github.com/coreos/rkt/common"
	"github.com/coreos/rkt/pkg/fileutil"
	stage1commontypes "github.com/coreos/rkt/stage1/common/types"
	"github.com/hashicorp/errwrap"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/syndtr/gocapability/capability"
)

// DefaultMountTable is the list of mounts that every application in the
// pod should have.
//
// If the application already has a bind mount configured for that destination,
// the entry in this table will be skipped.
//
// It bind-mounts the following paths from the stage1 to the stage2:
// - /sys
// - /proc
// - /dev/shm
// - /dev/pts
// - /run/systemd/journal
// - /proc/sys/kernel/hostname
// - /run/systemd/notify
var DefaultMountTable = []spec.Mount{
	{
		Type:        "proc",
		Source:      "proc",
		Destination: "/proc",
	},
	{
		Type:        "bind",
		Source:      "/sys",
		Destination: "/sys",
		Options:     []string{"bind"},
	},
	{
		Type:        "tmpfs",
		Source:      "tmpfs",
		Destination: "/dev",
		Options:     []string{"nosuid", "strictatime", "mode=755", "size=65536k"},
	},
	/* Every app in the pod should share shm and pts */
	{
		Type:        "bind",
		Source:      "/dev/shm",
		Destination: "/dev/shm",
		Options:     []string{"bind"},
	},
	{
		Type:        "bind",
		Source:      "/dev/pts",
		Destination: "/dev/pts",
		Options:     []string{"bind"},
	},
	/* Bind mount the journal in to the pod */
	{
		Type:        "bind",
		Source:      "/run/systemd/journal",
		Destination: "/run/systemd/journal",
		Options:     []string{"bind"},
	},
	/* Allow apps to notify systemd  - maybe not needed if
	https://github.com/systemd/systemd/issues/3544 is done. */
	{
		Type:        "bind",
		Source:      "/run/systemd/notify",
		Destination: "/run/systemd/notify",
		Options:     []string{"bind"},
	},
	{
		Type:        "bind",
		Source:      "/proc/sys/kernel/hostname",
		Destination: "/etc/hostname",
		Options:     []string{"bind"},
	},
}

// DefaultDevicePolicy is the device access policy applied to all containers
// without InsecureOptions.DisablePaths.
// It allows access to a standard set of devices. If any additional devices are
// bind-mounted in, then they will be added later
var DefaultDevicePolicy = []spec.DeviceCgroup{
	/* default deny */
	{
		Allow:  false,
		Access: sp("rwm"),
	},

	/* /dev/null */
	{
		Allow:  true,
		Access: sp("rwm"),
		Type:   sp("c"),
		Major:  i64p(0x1),
		Minor:  i64p(0x3),
	},
	/* /dev/zero */
	{
		Allow:  true,
		Access: sp("rwm"),
		Type:   sp("c"),
		Major:  i64p(0x1),
		Minor:  i64p(0x5),
	},
	/* /dev/full */
	{
		Allow:  true,
		Access: sp("rwm"),
		Type:   sp("c"),
		Major:  i64p(0x1),
		Minor:  i64p(0x7),
	},
	/* /dev/random */
	{
		Allow:  true,
		Access: sp("rwm"),
		Type:   sp("c"),
		Major:  i64p(0x1),
		Minor:  i64p(0x8),
	},
	/* /dev/urandom */
	{
		Allow:  true,
		Access: sp("rwm"),
		Type:   sp("c"),
		Major:  i64p(0x1),
		Minor:  i64p(0x9),
	},
	/* /dev/tty */
	{
		Allow:  true,
		Access: sp("rwm"),
		Type:   sp("c"),
		Major:  i64p(0x5),
		Minor:  i64p(0x0),
	},
	/* /dev/pts/* */
	{
		Allow:  true,
		Access: sp("rw"),
		Type:   sp("c"),
		Major:  i64p(0x88),
	},
}

// GenerateRuncSpec generates a runc runtime configuration for an application
func GenerateRuncSpec(pa *preparedApp, pod *stage1commontypes.Pod) (*spec.Spec, error) {
	ra := pa.app
	app := ra.App

	// Variables for spec.Process
	additionalGids := make([]uint32, 0, len(app.SupplementaryGIDs))
	for _, g := range app.SupplementaryGIDs {
		additionalGids = append(additionalGids, uint32(g))
	}

	env := make([]string, 0, len(pa.env))
	for _, envvar := range pa.env {
		env = append(env, fmt.Sprintf("%s=%s", envvar.Name, envvar.Value))
	}
	for k, v := range common.DefaultEnv {
		if _, exists := app.Environment.Get(k); !exists {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	// Use the same cgroup that systemd will put us on, but one level deeper,
	// because the runc binary itself will be one level up.
	cgroupsPath := fmt.Sprintf("/%s/system.slice/%s.service/app", pod.SubCgroupName, ra.Name.String())

	// Compute mounts.
	// Mounts from the user are handled by the stage1, but we don't want to
	// step on them, so we remove any mounts we'd otherwise generate if there
	// is already something on that destination.
	mounts := []spec.Mount{}
	userMPs := make(map[string]interface{})
	for _, m := range pa.mounts {
		userMPs[m.Mount.Path] = nil
	}

	for _, m := range DefaultMountTable {
		if err := maybeAddMount(pod.Root, ra.Name, userMPs, &mounts, m, false, false); err != nil {
			return nil, err
		}
	}
	/* mount network config files from pod to app */
	if err := addNetConfig(pod.Root, ra.Name, userMPs, &mounts); err != nil {
		return nil, err
	}

	runcSpec := spec.Spec{
		Version: spec.Version,
		Platform: spec.Platform{
			OS:   runtime.GOOS,
			Arch: runtime.GOARCH,
		},
		Process: spec.Process{
			Terminal: pod.Interactive && !pod.Mutable, // Mutable pods are always "interactive"
			User: spec.User{
				UID:            pa.uid,
				GID:            pa.gid,
				AdditionalGids: additionalGids,
			},
			Args:            app.Exec,
			Env:             env,
			Cwd:             app.WorkingDirectory,
			Capabilities:    pa.capabilities,
			NoNewPrivileges: getAppNoNewPrivileges(app.Isolators),
		},
		Root: spec.Root{
			Path:     "rootfs",
			Readonly: ra.ReadOnlyRootFS,
		},
		Hooks: spec.Hooks{
			// Add a prestart hook that does some simple filesystem cleanup.
			// User defined hooks are generated below
			Prestart: nil,
			/*[]spec.Hook{
				{
					Path: "/runc-prepare",
					Args: []string{"runc-prepare", common.RelAppRootfsPath(ra.Name)},
				},
			},*/
		},
		Mounts: mounts,
		Linux: &spec.Linux{
			Resources: &spec.Resources{
				Memory: &spec.Memory{},
				CPU:    &spec.CPU{},
			},
			ReadonlyPaths: pa.roPaths,
			MaskedPaths:   append(pa.hiddenDirs, pa.hiddenPaths...),
			Namespaces: []spec.Namespace{
				{
					Type: spec.MountNamespace,
				},
			},
			CgroupsPath: &cgroupsPath,
		},
	}

	/*
	 * Resources
	 */
	runcSpec.Linux.Resources.Memory.Limit = pa.resources.MemoryLimit

	if pa.resources.CPUQuota != nil {
		period := uint64(100000) // default cfs cpu period - 100ms
		runcSpec.Linux.Resources.CPU.Period = &period
		//cpu quota is a percentage value
		quota := period * *pa.resources.CPUQuota / 100
		runcSpec.Linux.Resources.CPU.Quota = &quota
	}
	runcSpec.Linux.Resources.CPU.Shares = pa.resources.LinuxCPUShares
	runcSpec.Linux.Resources.OOMScoreAdj = pa.resources.LinuxOOMScoreAdjust

	/*
	 * Hooks.
	 * Appc runtime hooks are paths in the container. Runc executes hooks in the
	 * "host"s (meaning stage1's) FS. The hook is otherwise in the container's
	 * namespaces. So, we need to chroot in to the stage2.
	 */
	for _, eh := range ra.App.EventHandlers {
		hook := spec.Hook{
			Path: "/bin/chroot", // relative to stage1rootfs
			Args: append([]string{"chroot", common.RelAppRootfsPath(ra.Name)},
				eh.Exec...),
			Env: env,
		}
		switch eh.Name {
		case "pre-start":
			runcSpec.Hooks.Prestart = append(runcSpec.Hooks.Prestart, hook)
		case "post-stop":
			runcSpec.Hooks.Poststop = append(runcSpec.Hooks.Poststop, hook)
		default:
			return nil, fmt.Errorf("Unknown hook type %s", eh.Name)
		}
	}

	if pod.InsecureOptions.DisableCapabilities {
		allCaps := []string{}
		for _, cap := range capability.List() {
			allCaps = append(allCaps, "CAP_"+strings.ToUpper(cap.String()))
		}
		runcSpec.Process.Capabilities = allCaps
	}

	if pod.InsecureOptions.DisablePaths {
		runcSpec.Linux.ReadonlyPaths = nil
		runcSpec.Linux.MaskedPaths = nil
		// Runc has a default device policy - so we need to allow all devices
		runcSpec.Linux.Resources.Devices = []spec.DeviceCgroup{
			{
				Allow:  true,
				Access: sp("rwm"),
			},
		}

	} else {
		// Without disablePaths, generate a restrictive device policy
		devs, err := runcDeviceAllows(pa)
		if err != nil {
			return nil, err
		}
		runcSpec.Linux.Resources.Devices = devs
	}

	if pa.seccomp != nil {
		seccomp := spec.Seccomp{}
		// The action for filtered syscalls
		var action spec.Action

		if pa.seccomp.mode == ModeBlacklist {
			seccomp.DefaultAction = spec.ActAllow
			action = spec.ActErrno
		} else if pa.seccomp.mode == ModeWhitelist {
			seccomp.DefaultAction = spec.ActErrno
			action = spec.ActAllow
		}

		for _, syscall := range pa.seccomp.syscalls {
			seccomp.Syscalls = append(seccomp.Syscalls, spec.Syscall{
				Name:   syscall,
				Action: action,
			})
		}

		runcSpec.Linux.Seccomp = &seccomp
	}

	return &runcSpec, nil
}

// maybeAddMount will add a mount to the list of mounts if the destination
// is not already a mountpoint and the source exists.
//
// skipMissing will skip if the source file does not exist - but be careful,
//		this is evaluated in the host's mount ns.
// skipPresent will skip if the destination file *does* exist - but you
//		probably don't want this
func maybeAddMount(podRoot string, appName types.ACName, skipPaths map[string]interface{}, mounts *[]spec.Mount, mount spec.Mount, skipMissing, skipPresent bool) error {
	if ok, _ := skipPaths[mount.Destination]; ok == true {
		diag.Printf("Skipping mount at %s, already mounted in user mounts", mount.Destination)
		return nil
	}

	// Skip if the source does not exist
	if skipMissing {
		if _, err := os.Stat(filepath.Join(common.Stage1RootfsPath(podRoot), mount.Source)); err != nil {
			if os.IsNotExist(err) {
				diag.Printf("Skipping mount at %s, source does not exist", mount.Destination)
				return nil
			} else {
				return err
			}
		}
	}

	for _, m := range *mounts {
		if m.Destination == mount.Destination {
			diag.Printf("Skipping mount at %s, already mounted", m.Destination)
			return nil
		}
	}

	// Skip if the destination exists in the app's rootfs
	if skipPresent {
		_, err := os.Stat(filepath.Join(common.AppRootfsPath(podRoot, appName), mount.Destination))
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if err == nil {
			diag.Printf("Skipping mount at %s, destination already exists", mount.Destination)
			return nil
		}
	}

	*mounts = append(*mounts, mount)
	return nil
}

// addNetConfig ensures that a sane network configuration is bind-mounted
// from the stage1 in to the stage2 as needed.
func addNetConfig(podRoot string, appName types.ACName, skipPaths map[string]interface{}, mounts *[]spec.Mount) error {

	// Add resolv.conf if it exists
	if err := maybeAddMount(podRoot, appName, skipPaths, mounts,
		spec.Mount{
			Type:        "bind",
			Source:      "/etc/rkt-resolv.conf",
			Destination: "/etc/resolv.conf",
			Options:     []string{"bind"},
		}, true, false); err != nil {
		return err
	}

	// Add hosts if it exists
	if err := maybeAddMount(podRoot, appName, skipPaths, mounts,
		spec.Mount{
			Type:        "bind",
			Source:      "/etc/rkt/rkt-hosts",
			Destination: "/etc/hosts",
			Options:     []string{"bind"},
		}, true, false); err != nil {
		return err
	}

	// Otherwise add fallback hosts
	if err := maybeAddMount(podRoot, appName, skipPaths, mounts,
		spec.Mount{
			Type:        "bind",
			Source:      "/etc/hosts-fallback",
			Destination: "/etc/hosts",
			Options:     []string{"bind"},
		}, true, true); err != nil {
		return err
	}
	return nil
}

// runcDeviceAllows computes the device section, adding permissions if
// any devices are bind-mounted in to the container
func runcDeviceAllows(pa *preparedApp) ([]spec.DeviceCgroup, error) {
	devices := append([]spec.DeviceCgroup{}, DefaultDevicePolicy...)
	// Find all bound-in devices
	for _, m := range pa.mounts {
		if m.Volume.Kind != "host" {
			continue
		}
		if !fileutil.IsDeviceNode(m.Volume.Source) {
			continue
		}
		kind, major, minor, err := fileutil.GetDeviceInfo(m.Volume.Source)
		if err != nil {
			return nil, errwrap.Wrap(errors.New("Could not get device info"), err)
		}
		kinds := string(kind)
		maj := int64(major)
		min := int64(minor)

		access := "r"
		if !m.ReadOnly {
			access += "w"
		}

		devices = append(devices, spec.DeviceCgroup{
			Allow:  true,
			Access: &access,
			Type:   &kinds,
			Major:  &maj,
			Minor:  &min,
		})
	}
	return devices, nil
}

func writeRuncSpec(podRoot string, appName types.ACName, spec *spec.Spec) error {
	path := filepath.Join(common.AppPath(podRoot, appName), "config.json")

	b, err := json.Marshal(spec)
	if err != nil {
		return errwrap.Wrapf("Could not generate config.json", err)
	}

	err = ioutil.WriteFile(path, b, 0644)
	if err != nil {
		return errwrap.Wrapf("Could not write config.json", err)
	}
	return nil
}

func sp(in string) *string {
	return &in
}
func i64p(in int64) *int64 {
	return &in
}

func runcEnabled() bool {
	return true
}

// AppRuncUnit creates the unit file for a runc app.
func (uw *UnitWriter) AppRuncUnit(pa *preparedApp, binPath string, opts []*unit.UnitOption) []*unit.UnitOption {

	spec, err := GenerateRuncSpec(pa, uw.p)
	if err != nil {
		uw.err = err
		return nil
	}

	if err := writeRuncSpec(uw.p.Root, pa.app.Name, spec); err != nil {
		uw.err = err
		return nil
	}

	execStartString := "/runc"
	if uw.p.Debug {
		execStartString += " --debug --log runc.out"
	}
	execStartString += " run --no-new-keyring " + pa.app.Name.String()

	opts = append(opts,
		unit.NewUnitOption("Service", "ExecStart", execStartString),
		// The working directory is the app path (the dir above the rootfs)
		unit.NewUnitOption("Service", "WorkingDirectory", common.RelAppPath(pa.app.Name)),
		unit.NewUnitOption("Service", "NoNewPrivileges", "false"),
		unit.NewUnitOption("Service", "KillMode", "mixed"),
		unit.NewUnitOption("Unit", "After", "systemd-journald.service"),
		unit.NewUnitOption("Unit", "Requires", "systemd-journald.service"),
		unit.NewUnitOption("Unit", "DefaultDependencies", "false"),
	)

	return opts
}
