// Copyright 2017 The rkt Authors
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
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/appc/spec/schema/types"
	"github.com/hashicorp/errwrap"
	"github.com/rkt/rkt/common"
	pkgPod "github.com/rkt/rkt/pkg/pod"
	"github.com/spf13/cobra"
)

var (
	cmdCp = &cobra.Command{
		Use:   "cp [--app=APPNAME] [--no-clobber] [UUID:]SRC_PATH [UUID]:DEST_DIR_PATH",
		Short: "Copy a file/directory to/from an app within a rkt pod",
		Long: `UUID should be the UUID of a running pod.

UUID should be specified once, either on the SRC_PATH or the DEST_DIR_PATH.`,
		Run: ensureSuperuser(runWrapper(runCp)),
	}
	flagCpAppName string
	flagNoClobber bool
)

func init() {
	cmdRkt.AddCommand(cmdCp)
	cmdCp.Flags().StringVar(&flagCpAppName, "app", "", "name of the app to copy to/from within the specified pod")
	cmdCp.Flags().BoolVar(&flagNoClobber, "no-clobber", false, "do not overwrite existing files")
}

func runCp(cmd *cobra.Command, args []string) (exit int) {
	if len(args) < 2 {
		cmd.Usage()
		return 254
	}

	srcUUID, srcPath := splitPath(args[0])
	destUUID, destPath := splitPath(args[1])

	if (srcUUID == "") == (destUUID == "") {
		stderr.Print("pod UUID for be specified for either SRC_PATH or DST_DIR_PATH")
		return 254
	}

	fullSrcPath, err := calculateFullPath(srcUUID, srcPath, false)
	if err != nil {
		stderr.PrintE("problem retrieving SRC_PATH", err)
		return 254
	}

	fullDestPath, err := calculateFullPath(destUUID, destPath, true)
	if err != nil {
		stderr.PrintE("problem retrieving DEST_DIR_PATH", err)
		return 254
	}

	err = copyFileOrDirectory(fullSrcPath, fullDestPath)
	if err != nil {
		stderr.PrintE("unable to copy file/directory", err)
		return 254
	}

	return 0
}

// splitPath splits a path into a UUID and path.
// Absolute paths and paths starting with '.' are handled paths without UUID.
// Other paths containing ':' are split into UUID and path.
func splitPath(path string) (string, string) {
	if filepath.IsAbs(path) || strings.HasPrefix(path, ".") {
		return "", path
	}

	parts := strings.SplitN(path, ":", 2)
	if len(parts) == 1 {
		return "", path
	}

	return parts[0], parts[1]
}

// calculateFullPath returns a validated path.
// If uuid is empty the provided path is validated and returned as is.
// If uuid is not empty, the stage2rootfs is calculate and the path is joined to stage2rootfs.
func calculateFullPath(uuid string, path string, expectDir bool) (string, error) {
	if uuid == "" {
		return validatePath(path, expectDir)
	}

	p, err := pkgPod.PodFromUUIDString(getDataDir(), uuid)
	if err != nil {
		return "", errwrap.Wrap(errors.New("problem retrieving pod"), err)
	}
	defer p.Close()

	if p.State() != pkgPod.Running {
		return "", fmt.Errorf("pod %q isn't currently running", p.UUID)
	}

	appName, err := getAppNameForCp(p)
	if err != nil {
		return "", errwrap.Wrap(errors.New("unable to determine app name"), err)
	}

	stage1RootFs := filepath.Join(p.Path(), "stage1/rootfs")
	stage2RootFs := filepath.Join(stage1RootFs, common.RelAppRootfsPath(*appName))

	return validatePath(filepath.Join(stage2RootFs, path), expectDir)
}

// validatePath checks whether the path exists and cleans the path.
// If checkDir is true, it also checks whether the path points to a directory.
func validatePath(path string, checkDir bool) (string, error) {
	cleanPath := filepath.Clean(path)

	info, err := os.Stat(cleanPath)
	if err != nil {
		return cleanPath, fmt.Errorf("path not found")
	}

	if checkDir && !info.IsDir() {
		return cleanPath, fmt.Errorf("path is not a directory")
	}

	return cleanPath, nil
}

// getAppNameForCp returns the app name to enter
// If one was supplied in the flags then it's simply returned
// If the PM contains a single app, that app's name is returned
// If the PM has multiple apps, the names are printed and an error is returned
func getAppNameForCp(p *pkgPod.Pod) (*types.ACName, error) {
	if flagCpAppName != "" {
		return types.NewACName(flagCpAppName)
	}

	// figure out the app name, or show a list if multiple are present
	_, m, err := p.PodManifest()
	if err != nil {
		return nil, errwrap.Wrap(errors.New("error reading pod manifest"), err)
	}

	switch len(m.Apps) {
	case 0:
		return nil, fmt.Errorf("pod contains zero apps")
	case 1:
		return &m.Apps[0].Name, nil
	default:
	}

	stderr.Print("pod contains multiple apps:")
	for _, ra := range m.Apps {
		stderr.Printf("\t%v", ra.Name)
	}

	return nil, fmt.Errorf("specify app using \"rkt cp --app= ...\"")
}

// copyFileOrDirectory copies a source file or directory to a destination directory.
func copyFileOrDirectory(srcPath string, destDirPath string) error {
	_, srcFilename := filepath.Split(srcPath)
	destPath := filepath.Join(destDirPath, srcFilename)

	return copyTree(srcPath, destPath)
}

// copyTree recursively copies a directory tree, preserving permissions.
// Symlinks are ignored and skipped.
func copyTree(src string, dst string) error {
	si, err := os.Stat(src)
	if err != nil {
		return err
	}

	if si.Mode()&os.ModeSymlink != 0 {
		return nil
	}

	if !si.IsDir() {
		return copyFile(src, dst)
	}

	_, err = os.Stat(dst)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}

		// Create the destination directory if it doesn't exist yet.
		err = os.MkdirAll(dst, si.Mode())
		if err != nil {
			return err
		}
	}

	entries, err := ioutil.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		err = copyTree(srcPath, dstPath)
		if err != nil {
			return err
		}
	}

	return nil
}

// copyFile copies the contents of the file named src to the file named by dst.
// The file will be created if it does not already exist.
// If the no-clobber flag is set and the destination file exists, the destination file is not
// changed. If the no-clobber flag is not set and the destination file exists, the content will be
// replaced by the content of the source file. Permissions of the src file are applied to the dst
// file.
func copyFile(src, dst string) error {
	if flagNoClobber {
		if _, err := os.Stat(dst); err == nil {
			return nil
		}
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}

	err = out.Sync()
	if err != nil {
		return err
	}

	si, err := os.Stat(src)
	if err != nil {
		return err
	}

	err = os.Chmod(dst, si.Mode())
	if err != nil {
		return err
	}

	return nil
}
