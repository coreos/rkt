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

	"github.com/appc/spec/aci"
	"github.com/appc/spec/schema"
	"github.com/coreos/rkt/common"
	"github.com/hashicorp/errwrap"
	"github.com/spf13/cobra"
)

var (
	cmdExport = &cobra.Command{
		Use:   "export UUID OUTPUT_ACI_FILE",
		Short: "Export an exited pod to an ACI file",
		Long: `UUID should be the uuid of an exited pod.

Note that currently only pods with a single app and started with the --no-overlay option are supported.`,
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
		stderr.Print("pod is not exited. Only exited pods are supported.")
		return 1
	}

	if p.usesOverlay() {
		stderr.Print("pod uses overlayfs. Only pods using the --no-overlay flag are supported.")
		return 1
	}

	var apps schema.AppList
	if apps, err = p.getApps(); err != nil {
		stderr.PrintE("problem getting pod's app list", err)
		return 1
	}

	if len(apps) != 1 {
		stderr.Print("pod has more than one app. Only pods with one app are supported.")
		return 1
	}

	root := common.AppPath(p.path(), apps[0].Name)
	if err = buildAci(root, aci); err != nil {
		stderr.PrintE("error building aci", err)
		return 1
	}

	return 0
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
