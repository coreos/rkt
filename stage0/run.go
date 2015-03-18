// Copyright 2014 CoreOS, Inc.
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
// Rocket is a reference implementation of the app container specification.
//
// Execution on Rocket is divided into a number of stages, and the `rkt`
// binary implements the first stage (stage 0)
//

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/coreos/rocket/Godeps/_workspace/src/github.com/appc/spec/schema"
	"github.com/coreos/rocket/Godeps/_workspace/src/github.com/appc/spec/schema/types"
	"github.com/coreos/rocket/cas"
	"github.com/coreos/rocket/common"
	"github.com/coreos/rocket/pkg/aci"
	"github.com/coreos/rocket/version"
)

const (
	envLockFd = "RKT_LOCK_FD"
)

// per-app configuration parameters required by Prepare
type PrepareAppConfig struct {
	// TODO(jonboulle): These images are partially-populated hashes, this should be clarified.
	Name       types.ACName   // unique name given to this app on rkt cli
	Image      types.Hash     // image backing this app
	ExecAppend []string       // any appendages to the app.exec lines
	Mounts     []schema.Mount // volume->mountpoint mapping for this app
}

// configuration parameters required by Prepare
type PrepareConfig struct {
	CommonConfig
	Stage1Image types.Hash         // stage1 image containing usable /init and /enter entrypoints
	AppConfigs  []PrepareAppConfig // constituent application images and per-app settings
	Volumes     []types.Volume     // list of volumes that rocket can provide to applications
	InheritEnv  bool               // inherit parent environment into all apps
	ExplicitEnv []string           // always set these environment variables for all the apps
}

// configuration parameters needed by Run
type RunConfig struct {
	CommonConfig
	PrivateNet           bool // container should have its own network stack
	SpawnMetadataService bool // launch metadata service
	LockFd               int  // lock file descriptor
	Interactive          bool // whether the container is interactive or not
}

// configuration shared by both Run and Prepare
type CommonConfig struct {
	Store         *cas.Store // store containing all of the configured application images
	ContainersDir string     // root directory for rocket containers
	Debug         bool
}

func init() {
	log.SetOutput(ioutil.Discard)
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

// Prepare sets up a filesystem for a container based on the given config.
func Prepare(cfg PrepareConfig, dir string, uuid *types.UUID) error {
	if cfg.Debug {
		log.SetOutput(os.Stderr)
	}

	log.Printf("Preparing stage1")
	if err := setupStage1Image(cfg, cfg.Stage1Image, dir); err != nil {
		return fmt.Errorf("error preparing stage1: %v", err)
	}
	log.Printf("Wrote filesystem to %s\n", dir)

	cm := schema.ContainerRuntimeManifest{
		ACKind: "ContainerRuntimeManifest",
		UUID:   *uuid, // TODO(vc): later appc spec omits uuid from the crm, this is a temp hack.
		Apps:   make(schema.AppList, 0),
	}

	v, err := types.NewSemVer(version.Version)
	if err != nil {
		return fmt.Errorf("error creating version: %v", err)
	}
	cm.ACVersion = *v

	for _, appConf := range cfg.AppConfigs {
		im, err := setupAppImage(cfg, appConf.Image, dir)
		if err != nil {
			return fmt.Errorf("error setting up image %s: %v", appConf.Image, err)
		}
		if im.App == nil {
			return fmt.Errorf("error: image %s has no app section", appConf.Image)
		}

	nextMountPoint:
		for _, mp := range im.App.MountPoints {
			for _, cfm := range appConf.Mounts {
				if cfm.MountPoint.Equals(mp.Name) {
					continue nextMountPoint
				}
			}
			return fmt.Errorf("error: app %q image %q requires mount %q", appConf.Name, appConf.Image, mp.Name)
		}

		a := schema.RuntimeApp{
			Name: appConf.Name,
			Image: schema.RuntimeImage{
				Name: im.Name,
				ID:   appConf.Image,
			},
			Annotations: im.Annotations,
			Mounts:      appConf.Mounts,
		}

		if len(appConf.ExecAppend) > 0 {
			a.App = im.App
			a.App.Exec = append(a.App.Exec, appConf.ExecAppend...)
		}

		if cfg.InheritEnv || len(cfg.ExplicitEnv) > 0 {
			if a.App == nil {
				a.App = im.App
			}
			MergeEnvs(&a.App.Environment, cfg.InheritEnv, cfg.ExplicitEnv)
		}

		cm.Apps = append(cm.Apps, a)
	}

	cm.Volumes = cfg.Volumes

	cdoc, err := json.Marshal(cm)
	if err != nil {
		return fmt.Errorf("error marshalling container manifest: %v", err)
	}

	log.Printf("Writing container manifest")
	fn := common.ContainerManifestPath(dir)
	if err := ioutil.WriteFile(fn, cdoc, 0700); err != nil {
		return fmt.Errorf("error writing container manifest: %v", err)
	}
	return nil
}

// Run actually runs the prepared container by exec()ing the stage1 init inside
// the container filesystem.
func Run(cfg RunConfig, dir string) {
	if err := os.Setenv(envLockFd, fmt.Sprintf("%v", cfg.LockFd)); err != nil {
		log.Fatalf("setting lock fd environment: %v", err)
	}

	if cfg.SpawnMetadataService {
		log.Print("Launching metadata svc")
		if err := launchMetadataService(cfg.Debug); err != nil {
			log.Printf("Failed to launch metadata svc: %v", err)
		}
	}

	log.Printf("Pivoting to filesystem %s", dir)
	if err := os.Chdir(dir); err != nil {
		log.Fatalf("failed changing to dir: %v", err)
	}

	ep, err := getStage1Entrypoint(dir, runEntrypoint)
	if err != nil {
		log.Fatalf("error determining init entrypoint: %v", err)
	}
	log.Printf("Execing %s", ep)

	args := []string{filepath.Join(common.Stage1RootfsPath(dir), ep)}
	if cfg.Debug {
		args = append(args, "--debug")
	}
	if cfg.PrivateNet {
		args = append(args, "--private-net")
	}
	if cfg.Interactive {
		args = append(args, "--interactive")
	}
	if err := syscall.Exec(args[0], args, os.Environ()); err != nil {
		log.Fatalf("error execing init: %v", err)
	}
}

// setupAppImage attempts to load the app image by the given hash from the store,
// verifies that the image matches the hash, and extracts the image into a
// directory in the given dir.
// It returns the ImageManifest that the image contains.
// TODO(jonboulle): tighten up the Hash type here; currently it is partially-populated (i.e. half-length sha512)
func setupAppImage(cfg PrepareConfig, img types.Hash, cdir string) (*schema.ImageManifest, error) {
	log.Println("Loading image", img.String())

	ad := common.AppImagePath(cdir, img)
	err := os.MkdirAll(ad, 0776)
	if err != nil {
		return nil, fmt.Errorf("error creating image directory: %v", err)
	}

	if err := aci.RenderACIWithImageID(img, ad, cfg.Store); err != nil {
		return nil, fmt.Errorf("error rendering app image: %v", err)
	}

	err = os.MkdirAll(filepath.Join(ad, "rootfs/tmp"), 0777)
	if err != nil {
		return nil, fmt.Errorf("error creating tmp directory: %v", err)
	}

	b, err := ioutil.ReadFile(common.ImageManifestPath(cdir, img))
	if err != nil {
		return nil, fmt.Errorf("error reading app manifest: %v", err)
	}
	var am schema.ImageManifest
	if err := json.Unmarshal(b, &am); err != nil {
		return nil, fmt.Errorf("error unmarshaling app manifest: %v", err)
	}

	return &am, nil
}

// setupStage1Image attempts to expand the image by the given hash as the stage1
func setupStage1Image(cfg PrepareConfig, img types.Hash, cdir string) error {
	s1 := common.Stage1ImagePath(cdir)
	if err := os.MkdirAll(s1, 0755); err != nil {
		return fmt.Errorf("error creating stage1 directory: %v", err)
	}
	if err := aci.RenderACIWithImageID(img, s1, cfg.Store); err != nil {
		return fmt.Errorf("error rendering stage1 ACI: %v", err)
	}
	return nil
}

func launchMetadataService(debug bool) error {
	// use socket activation protocol to avoid race-condition of
	// service becoming ready
	l, err := net.ListenTCP("tcp4", &net.TCPAddr{Port: common.MetadataServicePrvPort})
	if err != nil {
		if err.(*net.OpError).Err.(*os.SyscallError).Err == syscall.EADDRINUSE {
			// assume metadata-service is already running
			return nil
		}
		return err
	}

	defer l.Close()

	lf, err := l.File()
	if err != nil {
		return err
	}

	args := []string{"/proc/self/exe"}
	if debug {
		args = append(args, "--debug")
	}
	args = append(args, "metadata-service", "--no-idle")

	cmd := exec.Cmd{
		Path:       args[0],
		Args:       args,
		Env:        append(os.Environ(), "LISTEN_FDS=1"),
		ExtraFiles: []*os.File{lf},
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
	}
	return cmd.Start()
}
