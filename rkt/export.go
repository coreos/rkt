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
	"compress/gzip"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"

	"github.com/appc/spec/aci"
	"github.com/appc/spec/schema"
	"github.com/coreos/rkt/common"
	"github.com/coreos/rkt/common/overlay"
	"github.com/hashicorp/errwrap"

	"github.com/coreos/rkt/store"
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

func runExport(cmd *cobra.Command, args []string) (exit int) {
	if len(args) != 2 {
		cmd.Usage()
		return 1
	}

	aci := args[1]
	ext := filepath.Ext(aci)
	if ext != schema.ACIExtension {
		stderr.Printf("extension must be %s (given %s)", schema.ACIExtension, aci)
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

	if p.usesOverlay() {
		tmpMntDir := path.Join(getDataDir(), "tmp", fmt.Sprintf("rkt-export-%s", p.uuid))
		if err := os.MkdirAll(tmpMntDir, common.DefaultRegularDirPerm); err != nil {
			stderr.PrintE("unable to create temp directory", err)
			return 1
		}

		mntDir, err := mountOverlay(p, &app, tmpMntDir)
		if err != nil {
			stderr.PrintE(fmt.Sprintf("couldn't mount directory at %s", tmpMntDir), err)
		}
		defer func() {
			if err := syscall.Unmount(mntDir, 0); err != nil {
				stderr.PrintE(fmt.Sprintf("error unmounting directory %s", mntDir), err)
				exit = 1
			}
			if err := os.RemoveAll(tmpMntDir); err != nil {
				stderr.PrintE("problem removing temp directory", err)
				exit = 1
			}
		}()
	}

	root := common.AppPath(p.path(), app.Name)
	if err = buildAci(root, aci); err != nil {
		stderr.PrintE("error building aci", err)
		return 1
	}

	return 0
}

// mountOverlay mounts the app from the overlay-rendered pod to the destination directory.
func mountOverlay(pod *pod, app *schema.RuntimeApp, dest string) (string, error) {
	if _, err := os.Stat(dest); err != nil {
		return "", err
	}

	s, err := store.NewStore(getDataDir())
	if err != nil {
		return "", errwrap.Wrap(errors.New("cannot open store"), err)
	}

	treeStoreID, err := pod.getAppTreeStoreID(app.Name)
	if err != nil {
		return "", err
	}
	lower := s.GetTreeStoreRootFS(treeStoreID)
	imgDir := path.Join(path.Join(pod.path(), "overlay"), treeStoreID)
	if _, err := os.Stat(imgDir); err != nil {
		return "", err
	}
	upper := path.Join(imgDir, "upper", app.Name.String())
	if _, err := os.Stat(upper); err != nil {
		return "", err
	}
	work := path.Join(imgDir, "work", app.Name.String())
	if _, err := os.Stat(work); err != nil {
		return "", err
	}

	mntDir, err := overlay.Mount(&overlay.MountCfg{lower, upper, work, dest, ""})
	if err != nil {
		return "", errwrap.Wrap(errors.New("problem mounting overlayfs directory"), err)
	}

	return mntDir, nil
}

func buildAci(root, target string) (e error) {
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

	mpath := filepath.Join(root, aci.ManifestFile)
	b, err := ioutil.ReadFile(mpath)
	if err != nil {
		return errwrap.Wrap(errors.New("unable to read Image Manifest"), err)
	}
	var im schema.ImageManifest
	if err := im.UnmarshalJSON(b); err != nil {
		return errwrap.Wrap(errors.New("unable to load Image Manifest"), err)
	}
	iw := aci.NewImageWriter(im, tr)

	if err := filepath.Walk(root, aci.BuildWalker(root, iw, nil)); err != nil {
		return errwrap.Wrap(errors.New("error walking rootfs"), err)
	}

	if err = iw.Close(); err != nil {
		return errwrap.Wrap(fmt.Errorf("unable to close image %s", target), err)
	}

	return
}
