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

// #cgo LDFLAGS: -ldl
// #include <stdlib.h>
// #include <dlfcn.h>
// #include <sys/types.h>
//
// int
// my_sd_pid_get_owner_uid(void *f, pid_t pid, uid_t *uid)
// {
//   int (*sd_pid_get_owner_uid)(pid_t, uid_t *);
//
//   sd_pid_get_owner_uid = (int (*)(pid_t, uid_t *))f;
//   return sd_pid_get_owner_uid(pid, uid);
// }
//
// int
// my_sd_pid_get_unit(void *f, pid_t pid, char **unit)
// {
//   int (*sd_pid_get_unit)(pid_t, char **);
//
//   sd_pid_get_unit = (int (*)(pid_t, char **))f;
//   return sd_pid_get_unit(pid, unit);
// }
//
// int
// my_sd_pid_get_slice(void *f, pid_t pid, char **slice)
// {
//   int (*sd_pid_get_slice)(pid_t, char **);
//
//   sd_pid_get_slice = (int (*)(pid_t, char **))f;
//   return sd_pid_get_slice(pid, slice);
// }
//
import "C"

// this implements /init of stage1/nspawn+systemd

import (
	"crypto/rand"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/goaci/proj2aci"
	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/schema/types"
	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/coreos/go-systemd/util"
	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/godbus/dbus"
	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/godbus/dbus/introspect"

	"github.com/coreos/rkt/common"
	"github.com/coreos/rkt/common/cgroup"
	"github.com/coreos/rkt/networking"
	"github.com/coreos/rkt/pkg/sys"
)

const (
	// Path to systemd-nspawn binary within the stage1 rootfs
	nspawnBin = "/usr/bin/systemd-nspawn"
	// Path to the interpreter within the stage1 rootfs
	interpBin = "/usr/lib/ld-linux-x86-64.so.2"
	// Path to the localtime file/symlink in host
	localtimePath = "/etc/localtime"
)

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

var (
	debug       bool
	privNet     common.PrivateNetList
	interactive bool
)

func init() {
	flag.BoolVar(&debug, "debug", false, "Run in debug mode")
	flag.Var(&privNet, "private-net", "Setup private network")
	flag.BoolVar(&interactive, "interactive", false, "The pod is interactive")

	// this ensures that main runs only on main thread (thread group leader).
	// since namespace ops (unshare, setns) are done for a single thread, we
	// must ensure that the goroutine does not jump from OS thread to thread
	runtime.LockOSThread()
}

// machinedRegister checks if nspawn should register the pod to machined
func machinedRegister() bool {
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

func lookupPath(bin string, paths string) (string, error) {
	pathsArr := filepath.SplitList(paths)
	for _, path := range pathsArr {
		binPath := filepath.Join(path, bin)
		binAbsPath, err := filepath.Abs(binPath)
		if err != nil {
			return "", fmt.Errorf("unable to find absolute path for %s", binPath)
		}
		d, err := os.Stat(binAbsPath)
		if err != nil {
			continue
		}
		// Check the executable bit, inspired by os.exec.LookPath()
		if m := d.Mode(); !m.IsDir() && m&0111 != 0 {
			return binAbsPath, nil
		}
	}
	return "", fmt.Errorf("unable to find %q in %q", bin, paths)
}

func installAssets() error {
	systemctlBin, err := lookupPath("systemctl", os.Getenv("PATH"))
	if err != nil {
		return err
	}
	bashBin, err := lookupPath("bash", os.Getenv("PATH"))
	if err != nil {
		return err
	}
	// More paths could be added in that list if some Linux distributions install it in a different path
	// Note that we look in /usr/lib/... first because of the merge:
	// http://www.freedesktop.org/wiki/Software/systemd/TheCaseForTheUsrMerge/
	systemdShutdownBin, err := lookupPath("systemd-shutdown", "/usr/lib/systemd:/lib/systemd")
	if err != nil {
		return err
	}
	systemdBin, err := lookupPath("systemd", "/usr/lib/systemd:/lib/systemd")
	if err != nil {
		return err
	}

	assets := []string{}
	assets = append(assets, proj2aci.GetAssetString("/usr/lib/systemd/systemd", systemdBin))
	assets = append(assets, proj2aci.GetAssetString("/usr/bin/systemctl", systemctlBin))
	assets = append(assets, proj2aci.GetAssetString("/usr/bin/bash", bashBin))
	// systemd-shutdown has to be installed at the same path as on the host
	// because it depends on systemd build flag -DSYSTEMD_SHUTDOWN_BINARY_PATH=
	assets = append(assets, proj2aci.GetAssetString(systemdShutdownBin, systemdShutdownBin))

	return proj2aci.PrepareAssets(assets, "./stage1/rootfs/", nil)
}

// getArgsEnv returns the nspawn args and env according to the usr used
func getArgsEnv(p *Pod, flavor string, systemdStage1Version string, debug bool) ([]string, []string, error) {
	args := []string{}
	env := os.Environ()

	switch flavor {
	case "coreos":
		args = append(args, filepath.Join(common.Stage1RootfsPath(p.Root), interpBin))
		args = append(args, filepath.Join(common.Stage1RootfsPath(p.Root), nspawnBin))
		args = append(args, "--boot") // Launch systemd in the pod

		if machinedRegister() {
			args = append(args, fmt.Sprintf("--register=true"))
		} else {
			args = append(args, fmt.Sprintf("--register=false"))
		}

		// use only dynamic libraries provided in the image
		env = append(env, "LD_LIBRARY_PATH="+filepath.Join(common.Stage1RootfsPath(p.Root), "usr/lib"))

	case "src":
		args = append(args, filepath.Join(common.Stage1RootfsPath(p.Root), nspawnBin))
		args = append(args, "--boot") // Launch systemd in the pod

		switch systemdStage1Version {
		case "v215":
			lfd, err := common.GetRktLockFD()
			if err != nil {
				return nil, nil, err
			}
			args = append(args, fmt.Sprintf("--keep-fd=%v", lfd))
		case "v219":
			// --keep-fd is not needed thanks to
			// stage1/rootfs/usr_from_src/patches/v219/0005-nspawn-close-extra-fds-before-execing-init.patch
		default:
			// since systemd-nspawn v220 (commit 6b7d2e, "nspawn: close extra fds
			// before execing init"), fds remain open, so --keep-fd is not needed.
		}

		if machinedRegister() {
			args = append(args, fmt.Sprintf("--register=true"))
		} else {
			args = append(args, fmt.Sprintf("--register=false"))
		}

	case "host":
		hostNspawnBin, err := lookupPath("systemd-nspawn", os.Getenv("PATH"))
		if err != nil {
			return nil, nil, err
		}

		// Check dynamically which version is installed on the host
		// Support version >= 220
		versionBytes, err := exec.Command(hostNspawnBin, "--version").CombinedOutput()
		if err != nil {
			return nil, nil, fmt.Errorf("unable to probe %s version: %v", hostNspawnBin, err)
		}
		versionStr := strings.SplitN(string(versionBytes), "\n", 2)[0]
		var version int
		n, err := fmt.Sscanf(versionStr, "systemd %d", &version)
		if err != nil {
			return nil, nil, fmt.Errorf("cannot parse version: %q", versionStr)
		}
		if n != 1 || version < 220 {
			return nil, nil, fmt.Errorf("rkt needs systemd-nspawn >= 220. %s version not supported: %v", hostNspawnBin, versionStr)
		}

		// Copy systemd, bash, etc. in stage1 at run-time
		if err := installAssets(); err != nil {
			return nil, nil, fmt.Errorf("cannot install assets from the host: %v", err)
		}

		args = append(args, hostNspawnBin)
		args = append(args, "--boot") // Launch systemd in the pod
		args = append(args, fmt.Sprintf("--register=true"))

	default:
		return nil, nil, fmt.Errorf("unrecognized stage1 flavor: %q", flavor)
	}

	// link journal only if the host is running systemd and stage1 supports
	// linking
	if util.IsRunningSystemd() && systemdSupportsJournalLinking(systemdStage1Version) {
		// we write /etc/machine-id here because systemd-nspawn needs it to link
		// the container's journal to the host
		mPath := filepath.Join(common.Stage1RootfsPath(p.Root), "etc", "machine-id")
		mId := strings.Replace(p.UUID.String(), "-", "", -1)

		if err := ioutil.WriteFile(mPath, []byte(mId), 0644); err != nil {
			log.Fatalf("error writing /etc/machine-id: %v\n", err)
		}

		args = append(args, "--link-journal=try-host")
	}

	if !debug {
		args = append(args, "--quiet")             // silence most nspawn output (log_warning is currently not covered by this)
		env = append(env, "SYSTEMD_LOG_LEVEL=err") // silence log_warning too
	}

	keepUnit, err := isRunningFromUnitFile()
	if err != nil {
		return nil, nil, fmt.Errorf("error determining if we're running from a unit file: %v", err)
	}

	if keepUnit {
		args = append(args, "--keep-unit")
	}

	nsargs, err := p.PodToNspawnArgs()
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to generate nspawn args: %v", err)
	}
	args = append(args, nsargs...)

	// Arguments to systemd
	args = append(args, "--")
	args = append(args, "--default-standard-output=tty") // redirect all service logs straight to tty
	if !debug {
		args = append(args, "--log-target=null") // silence systemd output inside pod
		// TODO remove --log-level=warning when we update stage1 to systemd v222
		args = append(args, "--log-level=warning") // limit log output (systemd-shutdown ignores --log-target)
		args = append(args, "--show-status=0")     // silence systemd initialization status output
	}

	return args, env, nil
}

func withClearedCloExec(lfd int, f func() error) error {
	err := sys.CloseOnExec(lfd, false)
	if err != nil {
		return err
	}
	defer sys.CloseOnExec(lfd, true)

	return f()
}

func forwardedPorts(pod *Pod) ([]networking.ForwardedPort, error) {
	fps := []networking.ForwardedPort{}

	for _, ep := range pod.Manifest.Ports {
		n := ""
		fp := networking.ForwardedPort{}

		for _, a := range pod.Manifest.Apps {
			for _, p := range a.App.Ports {
				if p.Name == ep.Name {
					if n == "" {
						fp.Protocol = p.Protocol
						fp.HostPort = ep.HostPort
						fp.PodPort = p.Port
						n = a.Name.String()
					} else {
						return nil, fmt.Errorf("Ambiguous exposed port in PodManifest: %q and %q both define port %q", n, a.Name, p.Name)
					}
				}
			}
		}

		if n == "" {
			return nil, fmt.Errorf("Port name %q is not defined by any apps", ep.Name)
		}

		fps = append(fps, fp)
	}

	// TODO(eyakubovich): validate that there're no conflicts

	return fps, nil
}

func writePpid(pid int) error {
	// write ppid file as specified in
	// Documentation/devel/stage1-implementors-guide.md
	out, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("Cannot get current working directory: %v\n", err)
	}
	// we are the parent of the process that is PID 1 in the container so we write our PID to "ppid"
	err = ioutil.WriteFile(filepath.Join(out, "ppid"),
		[]byte(fmt.Sprintf("%d\n", pid)), 0644)
	if err != nil {
		return fmt.Errorf("Cannot write ppid file: %v\n", err)
	}
	return nil
}

func stage1() int {
	uuid, err := types.NewUUID(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, "UUID is missing or malformed")
		return 1
	}

	root := "."
	p, err := LoadPod(root, uuid)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load pod: %v\n", err)
		return 1
	}

	// set close-on-exec flag on RKT_LOCK_FD so it gets correctly closed when invoking
	// network plugins
	lfd, err := common.GetRktLockFD()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get rkt lock fd: %v\n", err)
		return 1
	}

	if err := sys.CloseOnExec(lfd, true); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to set FD_CLOEXEC on rkt lock: %v\n", err)
		return 1
	}

	mirrorLocalZoneInfo(p.Root)

	if privNet.Any() {
		fps, err := forwardedPorts(p)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return 6
		}

		n, err := networking.Setup(root, p.UUID, fps, privNet)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to setup network: %v\n", err)
			return 6
		}
		defer n.Teardown()

		if err = n.Save(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to save networking state %v\n", err)
			return 6
		}

		hostIP, err := n.GetDefaultHostIP()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get default Host IP: %v\n", err)
			return 6
		}

		mdsToken, err := generateMDSToken()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to generate MDS token: %v", err)
			return 8
		}

		p.MetadataServiceURL = common.MetadataServicePublicURL(hostIP, mdsToken)

		if err = registerPod(p, mdsToken); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to register pod: %v\n", err)
			return 8
		}

		defer unregisterPod(p)
	}

	flavor, systemdStage1Version, err := p.getFlavor()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get stage1 flavor: %v\n", err)
		return 3
	}

	if err = p.WritePrepareAppTemplate(systemdStage1Version); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write prepare-app service template: %v\n", err)
		return 2
	}

	if err = p.PodToSystemd(interactive); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to configure systemd: %v\n", err)
		return 2
	}

	args, env, err := getArgsEnv(p, flavor, systemdStage1Version, debug)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 3
	}

	appHashes := p.GetAppHashes()
	s1Root := common.Stage1RootfsPath(p.Root)
	machineID := p.GetMachineID()
	subcgroup, err := getContainerSubCgroup(machineID)
	if err == nil {
		if err := cgroup.CreateCgroups(s1Root, subcgroup, appHashes); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating cgroups: %v\n", err)
			return 5
		}
	} else {
		fmt.Fprintf(os.Stderr, "Continuing with per-app isolators disabled: %v\n", err)
	}

	var execFn func() error

	if privNet.Any() {
		cmd := exec.Cmd{
			Path:   args[0],
			Args:   args,
			Stdin:  os.Stdin,
			Stdout: os.Stdout,
			Stderr: os.Stderr,
			Env:    env,
		}
		execFn = func() error {
			err = cmd.Start()
			if err != nil {
				return fmt.Errorf("Failed to start nspawn: %v\n", err)
			}
			if err = writePpid(cmd.Process.Pid); err != nil {
				return err
			}
			return cmd.Wait()
		}
	} else {
		if err = writePpid(os.Getpid()); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return 4
		}
		execFn = func() error {
			return syscall.Exec(args[0], args, env)
		}
	}

	err = withClearedCloExec(lfd, execFn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to execute nspawn: %v\n", err)
		return 7
	}

	return 0
}

func getContainerSubCgroup(machineID string) (string, error) {
	var subcgroup string
	fromUnit, err := isRunningFromUnitFile()
	if err != nil {
		return "", fmt.Errorf("could not determine if we're running from a unit file: %v", err)
	}
	if fromUnit {
		slice, err := getSlice()
		if err != nil {
			return "", fmt.Errorf("could not get slice name: %v", err)
		}
		slicePath, err := common.SliceToPath(slice)
		if err != nil {
			return "", fmt.Errorf("could not convert slice name to path: %v", err)
		}
		unit, err := getUnitFileName()
		if err != nil {
			return "", fmt.Errorf("could not get unit name: %v", err)
		}
		subcgroup = filepath.Join(slicePath, unit, "system.slice")
	} else {
		if machinedRegister() {
			// we are not in the final cgroup yet: systemd-nspawn will move us
			// to the correct cgroup later during registration so we can't
			// look it up in /proc/self/cgroup
			escapedmID := strings.Replace(machineID, "-", "\\x2d", -1)
			machineDir := "machine-" + escapedmID + ".scope"
			subcgroup = filepath.Join("machine.slice", machineDir, "system.slice")
		} else {
			// when registration is disabled the container will be directly
			// under rkt's cgroup so we can look it up in /proc/self/cgroup
			ownCgroupPath, err := cgroup.GetOwnCgroupPath("name=systemd")
			if err != nil {
				return "", fmt.Errorf("could not get own cgroup path: %v", err)
			}
			subcgroup = filepath.Join(ownCgroupPath, "system.slice")
		}
	}

	return subcgroup, nil
}

func getUnitFileName() (unit string, err error) {
	libname := C.CString("libsystemd.so")
	defer C.free(unsafe.Pointer(libname))
	handle := C.dlopen(libname, C.RTLD_LAZY)
	if handle == nil {
		err = fmt.Errorf("error opening libsystemd.so")
		return
	}
	defer func() {
		if r := C.dlclose(handle); r != 0 {
			err = fmt.Errorf("error closing libsystemd.so")
		}
	}()

	sym := C.CString("sd_pid_get_unit")
	defer C.free(unsafe.Pointer(sym))
	sd_pid_get_unit := C.dlsym(handle, sym)
	if sd_pid_get_unit == nil {
		err = fmt.Errorf("error resolving sd_pid_get_unit function")
		return
	}

	var s string
	u := C.CString(s)
	defer C.free(unsafe.Pointer(u))

	ret := C.my_sd_pid_get_unit(sd_pid_get_unit, 0, &u)
	if ret < 0 {
		err = fmt.Errorf("error calling sd_pid_get_unit: %v", syscall.Errno(-ret))
		return
	}

	unit = C.GoString(u)
	return
}

func getSlice() (slice string, err error) {
	libname := C.CString("libsystemd.so")
	defer C.free(unsafe.Pointer(libname))
	handle := C.dlopen(libname, C.RTLD_LAZY)
	if handle == nil {
		err = fmt.Errorf("error opening libsystemd.so")
		return
	}
	defer func() {
		if r := C.dlclose(handle); r != 0 {
			err = fmt.Errorf("error closing libsystemd.so")
		}
	}()

	sym := C.CString("sd_pid_get_slice")
	defer C.free(unsafe.Pointer(sym))
	sd_pid_get_slice := C.dlsym(handle, sym)
	if sd_pid_get_slice == nil {
		err = fmt.Errorf("error resolving sd_pid_get_slice function")
		return
	}

	var s string
	sl := C.CString(s)
	defer C.free(unsafe.Pointer(sl))

	ret := C.my_sd_pid_get_slice(sd_pid_get_slice, 0, &sl)
	if ret < 0 {
		err = fmt.Errorf("error calling sd_pid_get_slice: %v", syscall.Errno(-ret))
		return
	}

	slice = C.GoString(sl)
	return
}

func isRunningFromUnitFile() (ret bool, err error) {
	libname := C.CString("libsystemd.so")
	defer C.free(unsafe.Pointer(libname))
	handle := C.dlopen(libname, C.RTLD_LAZY)
	if handle == nil {
		// we can't open libsystemd.so so we assume systemd is not
		// installed and we're not running from a unit file
		ret = false
		return
	}
	defer func() {
		if r := C.dlclose(handle); r != 0 {
			err = fmt.Errorf("error closing libsystemd.so")
		}
	}()

	sd_pid_get_owner_uid := C.dlsym(handle, C.CString("sd_pid_get_owner_uid"))
	if sd_pid_get_owner_uid == nil {
		err = fmt.Errorf("error resolving sd_pid_get_owner_uid function")
		return
	}

	var uid C.uid_t
	errno := C.my_sd_pid_get_owner_uid(sd_pid_get_owner_uid, 0, &uid)
	// when we're running from a unit file, sd_pid_get_owner_uid returns
	// ENOENT (systemd <220) or ENXIO (systemd >=220)
	switch {
	case errno >= 0:
		ret = false
		return
	case syscall.Errno(-errno) == syscall.ENOENT || syscall.Errno(-errno) == syscall.ENXIO:
		ret = true
		return
	default:
		err = fmt.Errorf("error calling sd_pid_get_owner_uid: %v", syscall.Errno(-errno))
		return
	}
}

func systemdSupportsJournalLinking(version string) bool {
	switch {
	case version == "v219":
		fallthrough
	case version == "v220":
		fallthrough
	case version == "master":
		return true
	default:
		return false
	}
}

func generateMDSToken() (string, error) {
	bytes := make([]byte, 16)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", bytes), nil
}

func main() {
	flag.Parse()

	if !debug {
		log.SetOutput(ioutil.Discard)
	}

	// move code into stage1() helper so defered fns get run
	os.Exit(stage1())
}
