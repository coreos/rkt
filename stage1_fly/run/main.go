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

package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/schema"
	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/schema/types"

	"github.com/coreos/rkt/common"
	"github.com/coreos/rkt/pkg/sys"
	stage1common "github.com/coreos/rkt/stage1/common"
	stage1commontypes "github.com/coreos/rkt/stage1/common/types"
)

const (
	flavor = "fly"
)

type flyMount struct {
	HostPath         string
	TargetPrefixPath string
	RelTargetPath    string
	Fs               string
	Flags            uintptr
}

type volumeMountTuple struct {
	V types.Volume
	M schema.Mount
}

var (
	debug bool

	discardNetlist common.NetList
	discardBool    bool
	discardString  string
)

func getHostMounts() (map[string]struct{}, error) {
	hostMounts := map[string]struct{}{}

	mi, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return nil, err
	}
	defer mi.Close()

	sc := bufio.NewScanner(mi)
	for sc.Scan() {
		var (
			discard    string
			mountPoint string
		)

		_, err := fmt.Sscanf(sc.Text(),
			"%s %s %s %s %s",
			&discard, &discard, &discard, &discard, &mountPoint,
		)
		if err != nil {
			return nil, err
		}

		hostMounts[mountPoint] = struct{}{}
	}
	if sc.Err() != nil {
		return nil, fmt.Errorf("problem parsing mountinfo: %v", sc.Err())
	}
	return hostMounts, nil
}

func init() {
	flag.BoolVar(&debug, "debug", false, "Run in debug mode")

	// The following flags need to be supported by stage1 according to
	// https://github.com/coreos/rkt/blob/master/Documentation/devel/stage1-implementors-guide.md
	// TODO: either implement functionality or give not implemented warnings
	flag.Var(&discardNetlist, "net", "Setup networking")
	flag.BoolVar(&discardBool, "interactive", true, "The pod is interactive")
	flag.StringVar(&discardString, "mds-token", "", "MDS auth token")
	flag.StringVar(&discardString, "local-config", common.DefaultLocalConfigDir, "Local config path")
}

func evaluateMounts(rfs string, app string, p *stage1commontypes.Pod) ([]flyMount, error) {
	imApp := p.Images[app].App
	namedVolumeMounts := map[types.ACName]volumeMountTuple{}

	for _, m := range p.Manifest.Apps[0].Mounts {
		_, exists := namedVolumeMounts[m.Volume]
		if exists {
			return nil, fmt.Errorf("duplicate mount given: %q", m.Volume)
		}
		namedVolumeMounts[m.Volume] = volumeMountTuple{M: m}
		log.Printf("Adding %+v", namedVolumeMounts[m.Volume])
	}

	// Merge command-line Mounts with ImageManifest's MountPoints
	for _, mp := range imApp.MountPoints {
		tuple, exists := namedVolumeMounts[mp.Name]
		switch {
		case exists && tuple.M.Path != mp.Path:
			return nil, fmt.Errorf("conflicting path information from mount and mountpoint %q", mp.Name)
		case !exists:
			namedVolumeMounts[mp.Name] = volumeMountTuple{M: schema.Mount{Volume: mp.Name, Path: mp.Path}}
			log.Printf("Adding %+v", namedVolumeMounts[mp.Name])
		}
	}

	// Insert the command-line Volumes
	for _, v := range p.Manifest.Volumes {
		// Check if we have a mount for this volume
		tuple, exists := namedVolumeMounts[v.Name]
		if !exists {
			return nil, fmt.Errorf("missing mount for volume %q", v.Name)
		} else if tuple.M.Volume != v.Name {
			// assertion regarding the implementation, should never happen
			return nil, fmt.Errorf("mismatched volume:mount pair: %q != %q", v.Name, tuple.M.Volume)
		}
		namedVolumeMounts[v.Name] = volumeMountTuple{V: v, M: tuple.M}
		log.Printf("Adding %+v", namedVolumeMounts[v.Name])
	}

	// Merge command-line Volumes with ImageManifest's MountPoints
	for _, mp := range imApp.MountPoints {
		// Check if we have a volume for this mountpoint
		tuple, exists := namedVolumeMounts[mp.Name]
		if !exists || tuple.V.Name == "" {
			return nil, fmt.Errorf("missing volume for mountpoint %q", mp.Name)
		}

		// If empty, fill in ReadOnly bit
		if tuple.V.ReadOnly == nil {
			v := tuple.V
			v.ReadOnly = &mp.ReadOnly
			namedVolumeMounts[mp.Name] = volumeMountTuple{M: tuple.M, V: v}
			log.Printf("Adding %+v", namedVolumeMounts[mp.Name])
		}
	}

	// Gather host mounts which we make MS_SHARED if passed as a volume source
	hostMounts, err := getHostMounts()
	if err != nil {
		return nil, fmt.Errorf("can't gather host mounts: %v", err)
	}

	argFlyMounts := []flyMount{}
	var flags uintptr = syscall.MS_BIND // TODO: allow optional | syscall.MS_REC
	for _, tuple := range namedVolumeMounts {
		if _, isHostMount := hostMounts[tuple.V.Source]; isHostMount {
			// Mark the host mount as SHARED so the container's changes to the mount are propagated to the host
			argFlyMounts = append(argFlyMounts,
				flyMount{"", "", tuple.V.Source, "none", syscall.MS_REC | syscall.MS_SHARED},
			)
		}
		argFlyMounts = append(argFlyMounts,
			flyMount{tuple.V.Source, rfs, tuple.M.Path, "none", flags},
		)

		if tuple.V.ReadOnly != nil && *tuple.V.ReadOnly {
			argFlyMounts = append(argFlyMounts,
				flyMount{"", rfs, tuple.M.Path, "none", flags | syscall.MS_REMOUNT | syscall.MS_RDONLY},
			)
		}
	}
	return argFlyMounts, nil
}

func stage1() int {
	uuid, err := types.NewUUID(flag.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "UUID is missing or malformed\n")
		return 1
	}

	root := "."
	p, err := stage1commontypes.LoadPod(root, uuid)
	if err != nil {
		fmt.Fprintf(os.Stderr, "can't load pod: %v\n", err)
		return 1
	}

	if len(p.Manifest.Apps) != 1 {
		fmt.Fprintf(os.Stderr, "flavor %q only supports 1 application per Pod for now.\n", flavor)
		return 1
	}

	lfd, err := common.GetRktLockFD()
	if err != nil {
		fmt.Fprintf(os.Stderr, "can't get rkt lock fd: %v\n", err)
		return 1
	}

	// set close-on-exec flag on RKT_LOCK_FD so it gets correctly closed after execution is finished
	if err := sys.CloseOnExec(lfd, true); err != nil {
		fmt.Fprintf(os.Stderr, "can't set FD_CLOEXEC on rkt lock: %v\n", err)
		return 1
	}

	envList := p.Manifest.Apps[0].App.Environment
	defaultEnv := []string{"PATH=/bin:/sbin:/usr/bin:/usr/local/bin"}
	env := make([]string, len(envList))
	for _, e := range envList {
		if _, exists := envList.Get(defaultEnv[0]); !exists {
			env = append(env, defaultEnv[0])
		}
		env = append(env, e.Name+"="+e.Value)
	}
	
	args := p.Manifest.Apps[0].App.Exec
	rfs := filepath.Join(common.AppPath(p.Root, p.Manifest.Apps[0].Name), "rootfs")

	argFlyMounts, err := evaluateMounts(rfs, string(p.Manifest.Apps[0].Name), p)
	if err != nil {
		fmt.Fprintf(os.Stderr, "can't evaluate mounts: %v\n", err)
		return 1
	}

	effectiveMounts := append(
		[]flyMount{
			{"", "", "/dev", "none", syscall.MS_REC | syscall.MS_SHARED},
			{"/dev", rfs, "/dev", "none", syscall.MS_BIND | syscall.MS_REC},

			{"", "", "/proc", "none", syscall.MS_REC | syscall.MS_SHARED},
			{"/proc", rfs, "/proc", "none", syscall.MS_BIND | syscall.MS_REC},

			{"", "", "/sys", "none", syscall.MS_REC | syscall.MS_SHARED},
			{"/sys", rfs, "/sys", "none", syscall.MS_BIND | syscall.MS_REC},

			{"tmpfs", rfs, "/tmp", "tmpfs", 0},
		},
		argFlyMounts...,
	)

	for _, mount := range effectiveMounts {
		var (
			err            error
			hostPathInfo   os.FileInfo
			targetPathInfo os.FileInfo
		)

		if strings.HasPrefix(mount.HostPath, "/") {
			if hostPathInfo, err = os.Stat(mount.HostPath); err != nil {
				fmt.Fprintf(os.Stderr, "stat of host directory %s: \n%v", mount.HostPath, err)
				return 1
			}
		} else {
			hostPathInfo = nil
		}

		absTargetPath := filepath.Join(mount.TargetPrefixPath, mount.RelTargetPath)
		if targetPathInfo, err = os.Stat(absTargetPath); err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "stat of target directory %s: \n%v\n", absTargetPath, err)
			return 1
		}

		switch {
		case targetPathInfo == nil:
			absTargetPathParent, _ := filepath.Split(absTargetPath)
			if err := os.MkdirAll(absTargetPathParent, 0700); err != nil {
				fmt.Fprintf(os.Stderr, "can't create directory %q: \n%v", absTargetPath, err)
				return 1
			}
			switch {
			case hostPathInfo == nil || hostPathInfo.IsDir():
				if err := os.Mkdir(absTargetPath, 0700); err != nil {
					fmt.Fprintf(os.Stderr, "can't create directory %q: \n%v", absTargetPath, err)
					return 1
				}
			case !hostPathInfo.IsDir():
				file, err := os.OpenFile(absTargetPath, os.O_CREATE, 0700)
				if err != nil {
					fmt.Fprintf(os.Stderr, "can't create file %q: \n%v\n", absTargetPath, err)
					return 1
				}
				file.Close()
			}
		case hostPathInfo != nil:
			switch {
			case hostPathInfo.IsDir() && !targetPathInfo.IsDir():
				fmt.Fprintf(os.Stderr, "can't mount:  %q is a directory while %q is not\n", mount.HostPath, absTargetPath)
				return 1
			case !hostPathInfo.IsDir() && targetPathInfo.IsDir():
				fmt.Fprintf(os.Stderr, "can't mount:  %q is not a directory while %q is\n", mount.HostPath, absTargetPath)
				return 1
			}
		}

		if err := syscall.Mount(mount.HostPath, absTargetPath, mount.Fs, mount.Flags, ""); err != nil {
			fmt.Fprintf(os.Stderr, "can't mount %q on %q with flags %v: %v\n", mount.HostPath, absTargetPath, mount.Flags, err)
			return 1
		}
	}

	if err = stage1common.WritePpid(os.Getpid()); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 4
	}

	log.Printf("Chroot to %q", rfs)
	if err := syscall.Chroot(rfs); err != nil {
		fmt.Fprintf(os.Stderr, "can't chroot: %v\n", err)
		return 1
	}

	if err := os.Chdir("/"); err != nil {
		fmt.Fprintf(os.Stderr, "can't change to root new directory: %v\n", err)
		return 1
	}

	log.Printf("Execing %q in %q", args, rfs)
	err = stage1common.WithClearedCloExec(lfd, func() error {
		return syscall.Exec(args[0], args, env)
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "can't execute %q: %v\n", args[0], err)
		return 7
	}

	return 0
}

func main() {
	flag.Parse()

	if !debug {
		log.SetOutput(ioutil.Discard)
	}

	// move code into stage1() helper so defered fns get run
	os.Exit(stage1())
}
