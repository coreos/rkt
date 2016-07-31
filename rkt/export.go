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
	"archive/tar"
	"bufio"
	"compress/gzip"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/appc/spec/aci"
	"github.com/appc/spec/schema"
	"github.com/appc/spec/schema/types"
	"github.com/coreos/rkt/common"
	"github.com/coreos/rkt/common/overlay"
	"github.com/coreos/rkt/pkg/user"
	"github.com/coreos/rkt/store/imagestore"
	"github.com/hashicorp/errwrap"
	"github.com/spf13/cobra"
)

var (
	cmdExport = &cobra.Command{
		Use:   "export UUID OUTPUT_ACI_FILE",
		Short: "Export an exited pod to an ACI file",
		Long: `UUID should be the uuid of an exited pod.

Note that currently only pods with a single app can be exported.`,
		Run: runWrapper(runExport),
	}
)

func init() {
	cmdRkt.AddCommand(cmdExport)
	cmdExport.Flags().BoolVar(&flagOverwriteACI, "overwrite", false, "overwrite output ACI")
}

func appHasMountpoints(podPath string, appName types.ACName) (bool, error) {
	appRootfs := common.AppRootfsPath(podPath, appName)
	// add a filepath separator so we don't match the appRootfs path
	appRootfs += string(filepath.Separator)

	mi, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return false, err
	}
	defer mi.Close()

	sc := bufio.NewScanner(mi)
	for sc.Scan() {
		line := sc.Text()
		lineResult := strings.Split(line, " ")
		if len(lineResult) < 7 {
			return false, fmt.Errorf("not enough fields from line %q: %+v", line, lineResult)
		}

		mp := lineResult[4]

		if strings.HasPrefix(mp, appRootfs) {
			return true, nil
		}
	}
	if err := sc.Err(); err != nil {
		return false, err
	}
	return false, nil
}

func runExport(cmd *cobra.Command, args []string) (exit int) {
	if len(args) != 2 {
		cmd.Usage()
		return 1
	}

	outACI := args[1]
	ext := filepath.Ext(outACI)
	if ext != schema.ACIExtension {
		stderr.Printf("extension must be %s (given %s)", schema.ACIExtension, outACI)
		return 1
	}

	p, err := getPodFromUUIDString(args[0])
	if err != nil {
		stderr.PrintE("problem retrieving pod", err)
		return 1
	}
	defer p.Close()

	if !p.isExited {
		stderr.Print("pod is not exited. Only exited pods can be exported")
		return 1
	}

	apps, err := p.getApps()
	if err != nil {
		stderr.PrintE("problem getting the pod's app list", err)
		return 1
	}

	if len(apps) != 1 {
		stderr.Printf("pod has %d apps. Only pods with one app can be exported", len(apps))
		return 1
	}
	app := apps[0]

	root := common.AppPath(p.path(), app.Name)
	manifestPath := filepath.Join(common.AppInfoPath(p.path(), app.Name), aci.ManifestFile)
	if p.usesOverlay() {
		tmpDir := filepath.Join(getDataDir(), "tmp")
		if err := os.MkdirAll(tmpDir, common.DefaultRegularDirPerm); err != nil {
			stderr.PrintE("unable to create temp directory", err)
			return 1
		}
		podDir, err := ioutil.TempDir(tmpDir, fmt.Sprintf("rkt-export-%s", p.uuid))
		if err != nil {
			stderr.PrintE("unable to create export temp directory", err)
			return 1
		}
		defer func() {
			if err := os.RemoveAll(podDir); err != nil {
				stderr.PrintE("problem removing temp directory", err)
				exit = 1
			}
		}()
		mntDir := filepath.Join(podDir, "rootfs")
		if err := os.Mkdir(mntDir, common.DefaultRegularDirPerm); err != nil {
			stderr.PrintE("unable to create rootfs directory inside temp directory", err)
			return 1
		}

		if err := mountOverlay(p, &app, mntDir); err != nil {
			stderr.PrintE(fmt.Sprintf("couldn't mount directory at %s", mntDir), err)
			return 1
		}
		defer func() {
			if err := syscall.Unmount(mntDir, 0); err != nil {
				stderr.PrintE(fmt.Sprintf("error unmounting directory %s", mntDir), err)
				exit = 1
			}
		}()
		root = podDir
	} else {
		hasMPs, err := appHasMountpoints(p.path(), app.Name)
		if err != nil {
			stderr.PrintE("error parsing mountpoints", err)
			return 1
		}
		if hasMPs {
			stderr.Printf("pod has remaining mountpoints. Only pods using overlayfs or with no mountpoints can be exported")
			return 1
		}
	}

	// Check for user namespace (--private-user), if in use get uidRange
	var uidRange *user.UidRange
	privUserFile := filepath.Join(p.path(), common.PrivateUsersPreparedFilename)
	privUserContent, err := ioutil.ReadFile(privUserFile)
	if err == nil {
		uidRange = user.NewBlankUidRange()
		// The file was found, save uid & gid shift and count
		if err := uidRange.Deserialize(privUserContent); err != nil {
			stderr.PrintE(fmt.Sprintf("problem deserializing the content of %s", common.PrivateUsersPreparedFilename), err)
			return 1
		}
	}

	if err = buildAci(root, manifestPath, outACI, uidRange); err != nil {
		stderr.PrintE("error building aci", err)
		return 1
	}
	return 0
}

// mountOverlay mounts the app from the overlay-rendered pod to the destination directory.
func mountOverlay(pod *pod, app *schema.RuntimeApp, dest string) error {
	if _, err := os.Stat(dest); err != nil {
		return err
	}

	s, err := imagestore.NewStore(getDataDir())
	if err != nil {
		return errwrap.Wrap(errors.New("cannot open store"), err)
	}

	ts, err := newTreeStore(s)
	if err != nil {
		return errwrap.Wrap(errors.New("cannot open treestore"), err)
	}

	treeStoreID, err := pod.getAppTreeStoreID(app.Name)
	if err != nil {
		return err
	}
	lower := ts.GetRootFS(treeStoreID)
	imgDir := filepath.Join(filepath.Join(pod.path(), "overlay"), treeStoreID)
	if _, err := os.Stat(imgDir); err != nil {
		return err
	}
	upper := filepath.Join(imgDir, "upper", app.Name.String())
	if _, err := os.Stat(upper); err != nil {
		return err
	}
	work := filepath.Join(imgDir, "work", app.Name.String())
	if _, err := os.Stat(work); err != nil {
		return err
	}

	if err := overlay.Mount(&overlay.MountCfg{lower, upper, work, dest, ""}); err != nil {
		return errwrap.Wrap(errors.New("problem mounting overlayfs directory"), err)
	}

	return nil
}

// buildAci builds a target aci from the root directory using any uid shift
// information from uidRange.
func buildAci(root, manifestPath, target string, uidRange *user.UidRange) (e error) {
	mode := os.O_CREATE | os.O_WRONLY
	if flagOverwriteACI {
		mode |= os.O_TRUNC
	} else {
		mode |= os.O_EXCL
	}
	aciFile, err := os.OpenFile(target, mode, 0644)
	if err != nil {
		if os.IsExist(err) {
			return errors.New("target file exists (try --overwrite)")
		} else {
			return errwrap.Wrap(fmt.Errorf("unable to open target %s", target), err)
		}
	}

	gw := gzip.NewWriter(aciFile)
	tr := tar.NewWriter(gw)

	defer func() {
		tr.Close()
		gw.Close()
		aciFile.Close()
		// e is implicitly assigned by the return statement. As defer runs
		// after return, but before actually returning, this works.
		if e != nil {
			os.Remove(target)
		}
	}()

	b, err := ioutil.ReadFile(manifestPath)
	if err != nil {
		return errwrap.Wrap(errors.New("unable to read Image Manifest"), err)
	}
	var im schema.ImageManifest
	if err := im.UnmarshalJSON(b); err != nil {
		return errwrap.Wrap(errors.New("unable to load Image Manifest"), err)
	}
	iw := aci.NewImageWriter(im, tr)

	// Unshift uid and gid when pod was started with --private-user (user namespace)
	var walkerCb aci.TarHeaderWalkFunc = func(hdr *tar.Header) bool {
		if uidRange != nil {
			uid, gid, err := uidRange.UnshiftRange(uint32(hdr.Uid), uint32(hdr.Gid))
			if err != nil {
				stderr.PrintE("error unshifting gid and uid", err)
				return false
			}
			hdr.Uid, hdr.Gid = int(uid), int(gid)
		}
		return true
	}

	if err := filepath.Walk(root, aci.BuildWalker(root, iw, walkerCb)); err != nil {
		return errwrap.Wrap(errors.New("error walking rootfs"), err)
	}

	if err = iw.Close(); err != nil {
		return errwrap.Wrap(fmt.Errorf("unable to close image %s", target), err)
	}

	return
}
