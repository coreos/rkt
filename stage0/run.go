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

package stage0

//
// rkt is a reference implementation of the app container specification.
//
// Execution on rkt is divided into a number of stages, and the `rkt`
// binary implements the first stage (stage 0)
//

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/schema"
	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/schema/types"
	"github.com/coreos/rkt/common"
	"github.com/coreos/rkt/common/apps"
	"github.com/coreos/rkt/pkg/aci"
	"github.com/coreos/rkt/pkg/fileutil"
	"github.com/coreos/rkt/pkg/label"
	"github.com/coreos/rkt/pkg/sys"
	"github.com/coreos/rkt/pkg/uid"
	"github.com/coreos/rkt/store"
	"github.com/coreos/rkt/version"
)

// configuration parameters required by Prepare
type PrepareConfig struct {
	CommonConfig
	Apps         *apps.Apps          // apps to prepare
	InheritEnv   bool                // inherit parent environment into apps
	ExplicitEnv  []string            // always set these environment variables for all the apps
	Volumes      []types.Volume      // list of volumes that rkt can provide to applications
	Ports        []types.ExposedPort // list of ports that rkt will expose on the host
	UseOverlay   bool                // prepare pod with overlay fs
	PodManifest  string              // use the pod manifest specified by the user, this will ignore flags such as '--volume', '--port', etc.
	PrivateUsers *uid.UidRange       // User namespaces
}

// configuration parameters needed by Run
type RunConfig struct {
	CommonConfig
	PrivateNet     common.PrivateNetList // pod should have its own network stack
	PrivateNetArgs common.PrivateNetArgs // arguments passed to private network's plugins
	LockFd         int                   // lock file descriptor
	Interactive    bool                  // whether the pod is interactive or not
	MDSRegister    bool                  // whether to register with metadata service or not
	Apps           schema.AppList        // applications (prepare gets them via Apps)
	LocalConfig    string                // Path to local configuration
}

// configuration shared by both Run and Prepare
type CommonConfig struct {
	Store        *store.Store // store containing all of the configured application images
	Stage1Image  types.Hash   // stage1 image containing usable /init and /enter entrypoints
	UUID         *types.UUID  // UUID of the pod
	Debug        bool
	MountLabel   string // selinux label to use for fs
	ProcessLabel string // selinux label to use for process
}

func init() {
	// this ensures that main runs only on main thread (thread group leader).
	// since namespace ops (unshare, setns) are done for a single thread, we
	// must ensure that the goroutine does not jump from OS thread to thread
	runtime.LockOSThread()
}

// MergeEnvs amends appEnv setting variables in setEnv before setting anything new from os.Environ if inheritEnv = true
// setEnv is expected to be in the os.Environ() key=value format
func MergeEnvs(appEnv *types.Environment, inheritEnv bool, setEnv []string) {
	for _, ev := range setEnv {
		pair := strings.SplitN(ev, "=", 2)
		appEnv.Set(pair[0], pair[1])
	}

	if inheritEnv {
		for _, ev := range os.Environ() {
			pair := strings.SplitN(ev, "=", 2)
			if _, exists := appEnv.Get(pair[0]); !exists {
				appEnv.Set(pair[0], pair[1])
			}
		}
	}
}

func imageNameToAppName(name types.ACIdentifier) (*types.ACName, error) {
	parts := strings.Split(name.String(), "/")
	last := parts[len(parts)-1]

	sn, err := types.SanitizeACName(last)
	if err != nil {
		return nil, err
	}

	return types.MustACName(sn), nil
}

// generatePodManifest creates the pod manifest from the command line input.
// It returns the pod manifest as []byte on success.
// This is invoked if no pod manifest is specified at the command line.
func generatePodManifest(cfg PrepareConfig, dir string) ([]byte, error) {
	pm := schema.PodManifest{
		ACKind: "PodManifest",
		Apps:   make(schema.AppList, 0),
	}

	v, err := types.NewSemVer(version.Version)
	if err != nil {
		return nil, fmt.Errorf("error creating version: %v", err)
	}
	pm.ACVersion = *v

	if err := cfg.Apps.Walk(func(app *apps.App) error {
		img := app.ImageID

		am, err := cfg.Store.GetImageManifest(img.String())
		if err != nil {
			return fmt.Errorf("error getting the manifest: %v", err)
		}
		appName, err := imageNameToAppName(am.Name)
		if err != nil {
			return fmt.Errorf("error converting image name to app name: %v", err)
		}
		if err := prepareAppImage(cfg, *appName, img, dir, cfg.UseOverlay); err != nil {
			return fmt.Errorf("error setting up image %s: %v", img, err)
		}
		if pm.Apps.Get(*appName) != nil {
			return fmt.Errorf("error: multiple apps with name %s", am.Name)
		}
		if am.App == nil {
			return fmt.Errorf("error: image %s has no app section", img)
		}
		ra := schema.RuntimeApp{
			// TODO(vc): leverage RuntimeApp.Name for disambiguating the apps
			Name: *appName,
			App:  am.App,
			Image: schema.RuntimeImage{
				Name: &am.Name,
				ID:   img,
			},
			Annotations: am.Annotations,
		}

		if execOverride := app.Exec; execOverride != "" {
			ra.App.Exec = []string{execOverride}
		}

		if execAppends := app.Args; execAppends != nil {
			ra.App.Exec = append(ra.App.Exec, execAppends...)
		}

		if cfg.InheritEnv || len(cfg.ExplicitEnv) > 0 {
			MergeEnvs(&ra.App.Environment, cfg.InheritEnv, cfg.ExplicitEnv)
		}
		pm.Apps = append(pm.Apps, ra)
		return nil
	}); err != nil {
		return nil, err
	}

	// TODO(jonboulle): check that app mountpoint expectations are
	// satisfied here, rather than waiting for stage1
	pm.Volumes = cfg.Volumes
	pm.Ports = cfg.Ports

	pmb, err := json.Marshal(pm)
	if err != nil {
		return nil, fmt.Errorf("error marshalling pod manifest: %v", err)
	}
	return pmb, nil
}

// validatePodManifest reads the user-specified pod manifest, prepares the app images
// and validates the pod manifest. If the pod manifest passes validation, it returns
// the manifest as []byte.
// TODO(yifan): More validation in the future.
func validatePodManifest(cfg PrepareConfig, dir string) ([]byte, error) {
	pmb, err := ioutil.ReadFile(cfg.PodManifest)
	if err != nil {
		return nil, fmt.Errorf("error reading pod manifest: %v", err)
	}
	var pm schema.PodManifest
	if err := json.Unmarshal(pmb, &pm); err != nil {
		return nil, fmt.Errorf("error unmarshaling pod manifest: %v", err)
	}

	appNames := make(map[types.ACName]struct{})
	for _, ra := range pm.Apps {
		img := ra.Image

		if img.ID.Empty() {
			return nil, fmt.Errorf("no image ID for app %q", ra.Name)
		}
		am, err := cfg.Store.GetImageManifest(img.ID.String())
		if err != nil {
			return nil, fmt.Errorf("error getting the image manifest from store: %v", err)
		}
		if err := prepareAppImage(cfg, ra.Name, img.ID, dir, cfg.UseOverlay); err != nil {
			return nil, fmt.Errorf("error setting up image %s: %v", img, err)
		}
		if _, ok := appNames[ra.Name]; ok {
			return nil, fmt.Errorf("multiple apps with same name %s", ra.Name)
		}
		appNames[ra.Name] = struct{}{}
		if ra.App == nil && am.App == nil {
			return nil, fmt.Errorf("no app section in the pod manifest or the image manifest")
		}
	}
	return pmb, nil
}

// Prepare sets up a pod based on the given config.
func Prepare(cfg PrepareConfig, dir string, uuid *types.UUID) error {
	log.Printf("Preparing stage1")
	if err := prepareStage1Image(cfg, cfg.Stage1Image, dir, cfg.UseOverlay); err != nil {
		return fmt.Errorf("error preparing stage1: %v", err)
	}

	var pmb []byte
	var err error
	if len(cfg.PodManifest) > 0 {
		pmb, err = validatePodManifest(cfg, dir)
	} else {
		pmb, err = generatePodManifest(cfg, dir)
	}
	if err != nil {
		return err
	}

	log.Printf("Writing pod manifest")
	fn := common.PodManifestPath(dir)
	if err := ioutil.WriteFile(fn, pmb, 0700); err != nil {
		return fmt.Errorf("error writing pod manifest: %v", err)
	}

	fn = path.Join(dir, common.Stage1IDFilename)
	if err := ioutil.WriteFile(fn, []byte(cfg.Stage1Image.String()), 0700); err != nil {
		return fmt.Errorf("error writing stage1 ID: %v", err)
	}

	if cfg.UseOverlay {
		// mark the pod as prepared with overlay
		f, err := os.Create(filepath.Join(dir, common.OverlayPreparedFilename))
		if err != nil {
			return fmt.Errorf("error writing overlay marker file: %v", err)
		}
		defer f.Close()
	}

	if cfg.PrivateUsers.Shift > 0 {
		// mark the pod as prepared for user namespaces
		uidrangeBytes := cfg.PrivateUsers.Serialize()

		if err := ioutil.WriteFile(filepath.Join(dir, common.PrivateUsersPreparedFilename), uidrangeBytes, 0700); err != nil {
			return fmt.Errorf("error writing userns marker file: %v", err)
		}
	}

	return nil
}

func preparedWithOverlay(dir string) (bool, error) {
	_, err := os.Stat(filepath.Join(dir, common.OverlayPreparedFilename))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	if !common.SupportsOverlay() {
		return false, fmt.Errorf("the pod was prepared with overlay but overlay is not supported")
	}

	return true, nil
}

func preparedWithPrivateUsers(dir string) (string, error) {
	bytes, err := ioutil.ReadFile(filepath.Join(dir, common.PrivateUsersPreparedFilename))
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}

	return string(bytes), nil
}

// Run mounts the right overlay filesystems and actually runs the prepared
// pod by exec()ing the stage1 init inside the pod filesystem.
func Run(cfg RunConfig, dir string) {
	useOverlay, err := preparedWithOverlay(dir)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	privateUsers, err := preparedWithPrivateUsers(dir)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	log.Printf("Setting up stage1")
	if err := setupStage1Image(cfg, cfg.Stage1Image, dir, useOverlay); err != nil {
		log.Fatalf("error setting up stage1: %v", err)
	}
	log.Printf("Wrote filesystem to %s\n", dir)

	for _, app := range cfg.Apps {
		if err := setupAppImage(cfg, app.Name, app.Image.ID, dir, useOverlay); err != nil {
			log.Fatalf("error setting up app image: %v", err)
		}
	}

	if err := os.Setenv(common.EnvLockFd, fmt.Sprintf("%v", cfg.LockFd)); err != nil {
		log.Fatalf("setting lock fd environment: %v", err)
	}

	log.Printf("Pivoting to filesystem %s", dir)
	if err := os.Chdir(dir); err != nil {
		log.Fatalf("failed changing to dir: %v", err)
	}

	ep, err := getStage1Entrypoint(dir, runEntrypoint)
	if err != nil {
		log.Fatalf("error determining init entrypoint: %v", err)
	}
	args := []string{filepath.Join(common.Stage1RootfsPath(dir), ep)}
	log.Printf("Execing %s", ep)

	if cfg.Debug {
		args = append(args, "--debug")
	}
	if cfg.PrivateNet.Any() {
		args = append(args, "--private-net="+cfg.PrivateNet.String())
		for _, s := range cfg.PrivateNetArgs.Strings() {
			args = append(args, fmt.Sprintf("--private-net-args=%s", s))
		}
	}
	if cfg.Interactive {
		args = append(args, "--interactive")
	}
	if len(privateUsers) > 0 {
		args = append(args, "--private-users="+privateUsers)
	}
	if cfg.MDSRegister {
		mdsToken, err := registerPod(".", cfg.UUID, cfg.Apps)
		if err != nil {
			log.Fatalf("failed to register the pod: %v", err)
		}

		args = append(args, "--mds-token="+mdsToken)
	}

	if cfg.LocalConfig != "" {
		args = append(args, "--local-config="+cfg.LocalConfig)
	}

	args = append(args, cfg.UUID.String())

	// make sure the lock fd stays open across exec
	if err := sys.CloseOnExec(cfg.LockFd, false); err != nil {
		log.Fatalf("error clearing FD_CLOEXEC on lock fd")
	}

	if err := label.SetProcessLabel(cfg.ProcessLabel); err != nil {
		log.Fatalf("error setting process SELinux label: %v", err)
	}

	if err := syscall.Exec(args[0], args, os.Environ()); err != nil {
		log.Fatalf("error execing init: %v", err)
	}
}

// prepareAppImage renders and verifies the tree cache of the app image that
// corresponds to the given app name.
// When useOverlay is false, it attempts to render and expand the app image
func prepareAppImage(cfg PrepareConfig, appName types.ACName, img types.Hash, cdir string, useOverlay bool) error {
	log.Println("Loading image", img.String())

	if useOverlay {
		if cfg.PrivateUsers.Shift > 0 {
			return fmt.Errorf("cannot use both overlay and user namespace: not implemented yet. (Try --no-overlay)")
		}
		if err := cfg.Store.RenderTreeStore(img.String(), false); err != nil {
			return fmt.Errorf("error rendering tree image: %v", err)
		}
		if err := cfg.Store.CheckTreeStore(img.String()); err != nil {
			log.Printf("Warning: tree cache is in a bad state. Rebuilding...")
			if err := cfg.Store.RenderTreeStore(img.String(), true); err != nil {
				return fmt.Errorf("error rendering tree image: %v", err)
			}
		}
	} else {
		ad := common.AppPath(cdir, appName)
		err := os.MkdirAll(ad, 0755)
		if err != nil {
			return fmt.Errorf("error creating image directory: %v", err)
		}

		if err := aci.RenderACIWithImageID(img, ad, cfg.Store, cfg.PrivateUsers); err != nil {
			return fmt.Errorf("error rendering ACI: %v", err)
		}
	}
	return nil
}

// setupAppImage mounts the overlay filesystem for the app image that
// corresponds to the given hash. Then, it creates the tmp directory.
// When useOverlay is false it just creates the tmp directory for this app.
func setupAppImage(cfg RunConfig, appName types.ACName, img types.Hash, cdir string, useOverlay bool) error {
	ad := common.AppPath(cdir, appName)
	if useOverlay {
		err := os.MkdirAll(ad, 0776)
		if err != nil {
			return fmt.Errorf("error creating image directory: %v", err)
		}

		if err := overlayRender(cfg, img, cdir, ad, appName.String()); err != nil {
			return fmt.Errorf("error rendering overlay filesystem: %v", err)
		}
	}

	return nil
}

// prepareStage1Image renders and verifies tree cache of the given hash
// when using overlay.
// When useOverlay is false, it attempts to render and expand the stage1.
func prepareStage1Image(cfg PrepareConfig, img types.Hash, cdir string, useOverlay bool) error {
	s1 := common.Stage1ImagePath(cdir)
	if err := os.MkdirAll(s1, 0755); err != nil {
		return fmt.Errorf("error creating stage1 directory: %v", err)
	}

	if err := cfg.Store.RenderTreeStore(img.String(), false); err != nil {
		return fmt.Errorf("error rendering tree image: %v", err)
	}
	if err := cfg.Store.CheckTreeStore(img.String()); err != nil {
		log.Printf("Warning: tree cache is in a bad state. Rebuilding...")
		if err := cfg.Store.RenderTreeStore(img.String(), true); err != nil {
			return fmt.Errorf("error rendering tree image: %v", err)
		}
	}

	if !useOverlay {
		if err := writeManifest(cfg.CommonConfig, img, s1); err != nil {
			return fmt.Errorf("error writing manifest: %v", err)
		}

		destRootfs := filepath.Join(s1, "rootfs")
		cachedTreePath := cfg.Store.GetTreeStoreRootFS(img.String())
		if err := fileutil.CopyTree(cachedTreePath, destRootfs, cfg.PrivateUsers); err != nil {
			return fmt.Errorf("error rendering ACI: %v", err)
		}
	}
	return nil
}

// setupStage1Image mounts the overlay filesystem for stage1.
// When useOverlay is false it is a noop
func setupStage1Image(cfg RunConfig, img types.Hash, cdir string, useOverlay bool) error {
	if useOverlay {
		s1 := common.Stage1ImagePath(cdir)
		// pass an empty appName: make sure it remains consistent with
		// overlayStatusDirTemplate
		if err := overlayRender(cfg, img, cdir, s1, ""); err != nil {
			return fmt.Errorf("error rendering overlay filesystem: %v", err)
		}

		// we will later read the status from the upper layer of the overlay fs
		// force the status directory to be there by touching it
		statusPath := filepath.Join(s1, "rootfs", "rkt", "status")
		if err := os.Chtimes(statusPath, time.Now(), time.Now()); err != nil {
			return fmt.Errorf("error touching status dir: %v", err)
		}
	}

	return nil
}

// writeManifest takes an img ID and writes the corresponding manifest in dest
func writeManifest(cfg CommonConfig, img types.Hash, dest string) error {
	manifest, err := cfg.Store.GetImageManifest(img.String())
	if err != nil {
		return err
	}

	mb, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("error marshalling image manifest: %v", err)
	}

	log.Printf("Writing image manifest")
	if err := ioutil.WriteFile(filepath.Join(dest, "manifest"), mb, 0700); err != nil {
		return fmt.Errorf("error writing image manifest: %v", err)
	}

	return nil
}

// overlayRender renders the image that corresponds to the given hash using the
// overlay filesystem.
// It writes the manifest in the specified directory and mounts an overlay
// filesystem from the cached tree of the image as rootfs.
func overlayRender(cfg RunConfig, img types.Hash, cdir string, dest string, appName string) error {
	if err := writeManifest(cfg.CommonConfig, img, dest); err != nil {
		return err
	}

	destRootfs := path.Join(dest, "rootfs")
	if err := os.MkdirAll(destRootfs, 0755); err != nil {
		return err
	}

	cachedTreePath := cfg.Store.GetTreeStoreRootFS(img.String())

	overlayDir := path.Join(cdir, "overlay", img.String())
	if err := os.MkdirAll(overlayDir, 0755); err != nil {
		return err
	}

	upperDir := path.Join(overlayDir, "upper", appName)
	if err := os.MkdirAll(upperDir, 0755); err != nil {
		return err
	}
	workDir := path.Join(overlayDir, "work", appName)
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return err
	}

	opts := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", cachedTreePath, upperDir, workDir)
	opts = label.FormatMountLabel(opts, cfg.MountLabel)
	if err := syscall.Mount("overlay", destRootfs, "overlay", 0, opts); err != nil {
		return fmt.Errorf("error mounting: %v", err)
	}

	return nil
}
