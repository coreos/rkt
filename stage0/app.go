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

//+build linux

package stage0

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/coreos/rkt/common"
	"github.com/coreos/rkt/common/apps"
	"github.com/coreos/rkt/pkg/lock"
	"github.com/coreos/rkt/pkg/user"
	// FIXME this should not be in stage1 anymore
	stage1types "github.com/coreos/rkt/stage1/common/types"

	"github.com/appc/spec/schema"
	"github.com/appc/spec/schema/types"
	"github.com/hashicorp/errwrap"
)

type StartConfig struct {
	*CommonConfig
	Dir         string
	UsesOverlay bool
	AppName     *types.ACName
	PodPID      int
}

type StopConfig struct {
	*CommonConfig
	Dir     string
	AppName *types.ACName
	PodPID  int
}

type AddConfig struct {
	*CommonConfig
	Image                types.Hash
	Apps                 *apps.Apps
	RktGid               int
	UsesOverlay          bool
	PodPath              string
	PodPID               int
	InsecureCapabilities bool
	InsecurePaths        bool
	InsecureSeccomp      bool
}

type RmConfig struct {
	*CommonConfig
	PodPath     string
	UsesOverlay bool
	AppName     *types.ACName
	PodPID      int
}

func AddApp(cfg AddConfig) error {
	// there should be only one app in the config
	app := cfg.Apps.Last()
	if app == nil {
		return errors.New("no image specified")
	}

	am, err := cfg.Store.GetImageManifest(cfg.Image.String())
	if err != nil {
		return err
	}

	var appName *types.ACName
	if app.Name != "" {
		appName, err = types.NewACName(app.Name)
		if err != nil {
			return err
		}
	} else {
		appName, err = imageNameToAppName(am.Name)
		if err != nil {
			return err
		}
	}

	debug("locking pod")
	l, err := lock.ExclusiveLock(common.PodManifestLockPath(cfg.PodPath), lock.RegFile)
	if err != nil {
		return errwrap.Wrap(errors.New("failed to lock pod"), err)
	}
	defer l.Close()

	p, err := stage1types.LoadPod(cfg.PodPath, cfg.UUID)
	if err != nil {
		return errwrap.Wrap(errors.New("error loading pod manifest"), err)
	}

	pm := p.Manifest

	var mutable bool
	ms, ok := pm.Annotations.Get("coreos.com/rkt/stage1/mutable")
	if ok {
		mutable, err = strconv.ParseBool(ms)
		if err != nil {
			return errwrap.Wrap(errors.New("error parsing mutable annotation"), err)
		}
	}

	if !mutable {
		return errors.New("immutable pod: cannot add application")
	}

	if pm.Apps.Get(*appName) != nil {
		return fmt.Errorf("error: multiple apps with name %s", *appName)
	}

	if am.App == nil && app.Exec == "" {
		return fmt.Errorf("error: image %s has no app section and --exec argument is not provided", cfg.Image)
	}

	appInfoDir := common.AppInfoPath(cfg.PodPath, *appName)
	if err := os.MkdirAll(appInfoDir, common.DefaultRegularDirPerm); err != nil {
		return errwrap.Wrap(errors.New("error creating apps info directory"), err)
	}

	pcfg := PrepareConfig{
		CommonConfig: cfg.CommonConfig,
		PrivateUsers: user.NewBlankUidRange(),
	}

	if cfg.UsesOverlay {
		privateUsers, err := preparedWithPrivateUsers(cfg.PodPath)
		if err != nil {
			log.FatalE("error reading user namespace information", err)
		}

		if err := pcfg.PrivateUsers.Deserialize([]byte(privateUsers)); err != nil {
			return err
		}
	}

	treeStoreID, err := prepareAppImage(pcfg, *appName, cfg.Image, cfg.PodPath, cfg.UsesOverlay)
	if err != nil {
		return errwrap.Wrap(fmt.Errorf("error preparing image %s", cfg.Image), err)
	}

	rcfg := RunConfig{
		CommonConfig: cfg.CommonConfig,
		UseOverlay:   cfg.UsesOverlay,
		RktGid:       cfg.RktGid,
	}

	if err := setupAppImage(rcfg, *appName, cfg.Image, cfg.PodPath, cfg.UsesOverlay); err != nil {
		return fmt.Errorf("error setting up app image: %v", err)
	}

	if cfg.UsesOverlay {
		imgDir := filepath.Join(cfg.PodPath, "overlay", treeStoreID)
		if err := os.Chown(imgDir, -1, cfg.RktGid); err != nil {
			return err
		}
	}

	ra := schema.RuntimeApp{
		Name: *appName,
		App:  am.App,
		Image: schema.RuntimeImage{
			Name:   &am.Name,
			ID:     cfg.Image,
			Labels: am.Labels,
		},
		Mounts:         MergeMounts(cfg.Apps.Mounts, app.Mounts),
		ReadOnlyRootFS: app.ReadOnlyRootFS,
	}

	if app.Exec != "" {
		// Create a minimal App section if not present
		if am.App == nil {
			ra.App = &types.App{
				User:  strconv.Itoa(os.Getuid()),
				Group: strconv.Itoa(os.Getgid()),
			}
		}
		ra.App.Exec = []string{app.Exec}
	}

	if app.Args != nil {
		ra.App.Exec = append(ra.App.Exec, app.Args...)
	}

	if app.WorkingDir != "" {
		ra.App.WorkingDirectory = app.WorkingDir
	}

	if err := prepareIsolators(app, ra.App); err != nil {
		return err
	}

	if app.User != "" {
		ra.App.User = app.User
	}

	if app.Group != "" {
		ra.App.Group = app.Group
	}

	if app.SupplementaryGIDs != nil {
		ra.App.SupplementaryGIDs = app.SupplementaryGIDs
	}

	if app.UserAnnotations != nil {
		ra.App.UserAnnotations = app.UserAnnotations
	}

	if app.UserLabels != nil {
		ra.App.UserLabels = app.UserLabels
	}

	if app.Environments != nil {
		envs := make([]string, 0, len(app.Environments))
		for name, value := range app.Environments {
			envs = append(envs, fmt.Sprintf("%s=%s", name, value))
		}
		// Let the app level environment override the environment variables.
		mergeEnvs(&ra.App.Environment, envs, true)
	}

	env := ra.App.Environment

	env.Set("AC_APP_NAME", appName.String())
	envFilePath := filepath.Join(common.Stage1RootfsPath(cfg.PodPath), "rkt", "env", appName.String())

	if err := common.WriteEnvFile(env, pcfg.PrivateUsers, envFilePath); err != nil {
		return err
	}

	apps := append(p.Manifest.Apps, ra)
	p.Manifest.Apps = apps

	if err := updatePodManifest(cfg.PodPath, p.Manifest); err != nil {
		return err
	}

	eep, err := getStage1Entrypoint(cfg.PodPath, enterEntrypoint)
	if err != nil {
		return errwrap.Wrap(errors.New("error determining 'enter' entrypoint"), err)
	}

	args := []string{
		cfg.UUID.String(),
		appName.String(),
		filepath.Join(common.Stage1RootfsPath(cfg.PodPath), eep),
		strconv.Itoa(cfg.PodPID),
	}

	if cfg.InsecureCapabilities {
		args = append(args, "--disable-capabilities-restriction")
	}

	if cfg.InsecurePaths {
		args = append(args, "--disable-paths")
	}

	if cfg.InsecureSeccomp {
		args = append(args, "--disable-seccomp")
	}

	privateUsers, err := preparedWithPrivateUsers(cfg.PodPath)
	if err != nil {
		log.FatalE("error reading user namespace information", err)
	}

	if privateUsers != "" {
		args = append(args, fmt.Sprintf("--private-users=%s", privateUsers))
	}

	if _, err := os.Create(common.AppCreatedPath(p.Root, appName.String())); err != nil {
		return err
	}

	if err := callEntrypoint(cfg.PodPath, appAddEntrypoint, args); err != nil {
		return err
	}

	return nil
}

func updatePodManifest(dir string, newPodManifest *schema.PodManifest) error {
	pmb, err := json.Marshal(newPodManifest)
	if err != nil {
		return err
	}

	debug("Writing pod manifest")
	return updateFile(common.PodManifestPath(dir), pmb)
}

func updateFile(path string, contents []byte) error {
	fi, err := os.Stat(path)
	if err != nil {
		return err
	}

	f, err := ioutil.TempFile(filepath.Dir(path), "")
	if err != nil {
		return err
	}
	defer f.Close()

	if err := f.Chmod(fi.Mode().Perm()); err != nil {
		return err
	}

	if _, err := f.Write(contents); err != nil {
		return errwrap.Wrap(errors.New("error writing to temp file"), err)
	}

	if err := os.Rename(f.Name(), path); err != nil {
		return err
	}

	return nil
}

func callEntrypoint(dir, entrypoint string, args []string) error {
	previousDir, err := os.Getwd()
	if err != nil {
		return err
	}

	debug("Pivoting to filesystem %s", dir)
	if err := os.Chdir(dir); err != nil {
		return errwrap.Wrap(errors.New("failed changing to dir"), err)
	}

	ep, err := getStage1Entrypoint(dir, entrypoint)
	if err != nil {
		return fmt.Errorf("%q not implemented for pod's stage1: %v", entrypoint, err)
	}
	execArgs := []string{filepath.Join(common.Stage1RootfsPath(dir), ep)}
	debug("Execing %s", ep)
	execArgs = append(execArgs, args...)

	c := exec.Cmd{
		Path:   execArgs[0],
		Args:   execArgs,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}

	if err := c.Run(); err != nil {
		return fmt.Errorf("error executing stage1's entrypoint %q: %v", entrypoint, err)
	}

	if err := os.Chdir(previousDir); err != nil {
		return errwrap.Wrap(errors.New("failed changing to dir"), err)
	}

	return nil
}

func RmApp(cfg RmConfig) error {
	debug("locking pod")
	l, err := lock.ExclusiveLock(common.PodManifestLockPath(cfg.PodPath), lock.RegFile)
	if err != nil {
		return errwrap.Wrap(errors.New("failed to lock pod"), err)
	}
	defer l.Close()

	p, err := stage1types.LoadPod(cfg.PodPath, cfg.UUID)
	if err != nil {
		return errwrap.Wrap(errors.New("error loading pod manifest"), err)
	}

	pm := p.Manifest

	var mutable bool
	ms, ok := pm.Annotations.Get("coreos.com/rkt/stage1/mutable")
	if ok {
		mutable, err = strconv.ParseBool(ms)
		if err != nil {
			return errwrap.Wrap(errors.New("error parsing mutable annotation"), err)
		}
	}

	if !mutable {
		return errors.New("immutable pod: cannot remove application")
	}

	app := pm.Apps.Get(*cfg.AppName)
	if app == nil {
		return fmt.Errorf("error: nonexistent app %q", *cfg.AppName)
	}

	treeStoreID, err := ioutil.ReadFile(common.AppTreeStoreIDPath(cfg.PodPath, *cfg.AppName))
	if err != nil {
		return err
	}

	eep, err := getStage1Entrypoint(cfg.PodPath, enterEntrypoint)
	if err != nil {
		return errwrap.Wrap(errors.New("error determining 'enter' entrypoint"), err)
	}

	if cfg.PodPID > 0 {
		// Call app-stop and app-rm entrypoint only if the pod is still running.
		// Otherwise, there's not much we can do about it except unmounting/removing
		// the file system.
		args := []string{
			cfg.UUID.String(),
			cfg.AppName.String(),
			filepath.Join(common.Stage1RootfsPath(cfg.PodPath), eep),
			strconv.Itoa(cfg.PodPID),
		}

		if err := callEntrypoint(cfg.PodPath, appStopEntrypoint, args); err != nil {
			status, err := common.GetExitStatus(err)
			// ignore nonexistent units failing to stop. Exit status 5
			// comes from systemctl and means the unit doesn't exist
			if err != nil {
				return err
			} else if status != 5 {
				return fmt.Errorf("exit status %d", status)
			}
		}

		if err := callEntrypoint(cfg.PodPath, appRmEntrypoint, args); err != nil {
			return err
		}
	}

	appInfoDir := common.AppInfoPath(cfg.PodPath, *cfg.AppName)
	if err := os.RemoveAll(appInfoDir); err != nil {
		return errwrap.Wrap(errors.New("error removing app info directory"), err)
	}

	if cfg.UsesOverlay {
		appRootfs := common.AppRootfsPath(cfg.PodPath, *cfg.AppName)
		if err := syscall.Unmount(appRootfs, 0); err != nil {
			return err
		}

		ts := filepath.Join(cfg.PodPath, "overlay", string(treeStoreID))
		if err := os.RemoveAll(ts); err != nil {
			return errwrap.Wrap(errors.New("error removing app info directory"), err)
		}
	}

	if err := os.RemoveAll(common.AppPath(cfg.PodPath, *cfg.AppName)); err != nil {
		return err
	}

	appStatusPath := filepath.Join(common.Stage1RootfsPath(cfg.PodPath), "rkt", "status", cfg.AppName.String())
	if err := os.Remove(appStatusPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	envPath := filepath.Join(common.Stage1RootfsPath(cfg.PodPath), "rkt", "env", cfg.AppName.String())
	if err := os.Remove(envPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	removeAppFromPodManifest(pm, cfg.AppName)

	if err := updatePodManifest(cfg.PodPath, pm); err != nil {
		return err
	}

	return nil
}

func removeAppFromPodManifest(pm *schema.PodManifest, appName *types.ACName) {
	for i, app := range pm.Apps {
		if app.Name == *appName {
			pm.Apps = append(pm.Apps[:i], pm.Apps[i+1:]...)
		}
	}
}

func StartApp(cfg StartConfig) error {
	p, err := stage1types.LoadPod(cfg.Dir, cfg.UUID)
	if err != nil {
		return errwrap.Wrap(errors.New("error loading pod manifest"), err)
	}

	pm := p.Manifest

	var mutable bool
	ms, ok := pm.Annotations.Get("coreos.com/rkt/stage1/mutable")
	if ok {
		mutable, err = strconv.ParseBool(ms)
		if err != nil {
			return errwrap.Wrap(errors.New("error parsing mutable annotation"), err)
		}
	}

	if !mutable {
		return errors.New("immutable pod: cannot start application")
	}

	app := pm.Apps.Get(*cfg.AppName)
	if app == nil {
		return fmt.Errorf("error: nonexistent app %q", *cfg.AppName)
	}

	eep, err := getStage1Entrypoint(cfg.Dir, enterEntrypoint)
	if err != nil {
		return errwrap.Wrap(errors.New("error determining 'enter' entrypoint"), err)
	}

	args := []string{
		cfg.UUID.String(),
		cfg.AppName.String(),
		filepath.Join(common.Stage1RootfsPath(cfg.Dir), eep),
		strconv.Itoa(cfg.PodPID),
	}

	if _, err := os.Create(common.AppStartedPath(p.Root, cfg.AppName.String())); err != nil {
		log.FatalE(fmt.Sprintf("error creating %s-started file", cfg.AppName.String()), err)
	}

	if err := callEntrypoint(cfg.Dir, appStartEntrypoint, args); err != nil {
		return err
	}

	return nil
}

func StopApp(cfg StopConfig) error {
	p, err := stage1types.LoadPod(cfg.Dir, cfg.UUID)
	if err != nil {
		return errwrap.Wrap(errors.New("error loading pod manifest"), err)
	}

	pm := p.Manifest

	var mutable bool
	ms, ok := pm.Annotations.Get("coreos.com/rkt/stage1/mutable")
	if ok {
		mutable, err = strconv.ParseBool(ms)
		if err != nil {
			return errwrap.Wrap(errors.New("error parsing mutable annotation"), err)
		}
	}

	if !mutable {
		return errors.New("immutable pod: cannot start application")
	}

	app := pm.Apps.Get(*cfg.AppName)
	if app == nil {
		return fmt.Errorf("error: nonexistent app %q", *cfg.AppName)
	}

	eep, err := getStage1Entrypoint(cfg.Dir, enterEntrypoint)
	if err != nil {
		return errwrap.Wrap(errors.New("error determining 'enter' entrypoint"), err)
	}

	args := []string{
		cfg.UUID.String(),
		cfg.AppName.String(),
		filepath.Join(common.Stage1RootfsPath(cfg.Dir), eep),
		strconv.Itoa(cfg.PodPID),
	}

	if err := callEntrypoint(cfg.Dir, appStopEntrypoint, args); err != nil {
		status, err := common.GetExitStatus(err)
		// exit status 5 comes from systemctl and means the unit doesn't exist
		if status == 5 {
			return fmt.Errorf("app %q is not running", app.Name)
		}

		return err
	}

	return nil
}
