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
	"archive/tar"
	"crypto/sha512"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/coreos/rocket/Godeps/_workspace/src/code.google.com/p/go-uuid/uuid"
	"github.com/coreos/rocket/Godeps/_workspace/src/github.com/appc/spec/schema"
	"github.com/coreos/rocket/Godeps/_workspace/src/github.com/appc/spec/schema/types"
	"github.com/coreos/rocket/cas"
	"github.com/coreos/rocket/common"
	"github.com/coreos/rocket/pkg/lock"
	ptar "github.com/coreos/rocket/pkg/tar"
	"github.com/coreos/rocket/version"
)

const (
	envLockFd = "RKT_LOCK_FD"
)

type Config struct {
	Store         *cas.Store // store containing all of the configured application images
	ContainersDir string     // root directory for rocket containers
	Stage1Image   types.Hash // stage1 image containing usable /init and /enter entrypoints
	Debug         bool
	// TODO(jonboulle): These images are partially-populated hashes, this should be clarified.
	Images           []types.Hash   // application images
	Volumes          []types.Volume // list of volumes that rocket can provide to applications
	PrivateNet       bool           // container should have its own network stack
	SpawnMetadataSvc bool           // launch metadata service
}

func init() {
	log.SetOutput(ioutil.Discard)
}

// Setup sets up a filesystem for a container based on the given config.
// The directory containing the filesystem is returned, and any error encountered.
func Setup(cfg Config) (string, error) {
	if cfg.Debug {
		log.SetOutput(os.Stderr)
	}

	if err := os.MkdirAll(cfg.ContainersDir, 0700); err != nil {
		return "", fmt.Errorf("error creating containers directory: %v", err)
	}

	// Create a unique directory for this container
	cuuid, dir, err := makeUniqueContainer(cfg.ContainersDir)
	if err != nil {
		return "", fmt.Errorf("error creating directory: %v", err)
	}

	// Set up the container lock
	if err := lockDir(dir); err != nil {
		return "", err
	}

	log.Printf("Preparing stage1")
	if err := setupStage1Image(cfg, cfg.Stage1Image, dir); err != nil {
		return "", fmt.Errorf("error preparing stage1: %v", err)
	}
	log.Printf("Wrote filesystem to %s\n", dir)

	cm := schema.ContainerRuntimeManifest{
		ACKind: "ContainerRuntimeManifest",
		UUID:   *cuuid,
		Apps:   make(schema.AppList, 0),
	}

	v, err := types.NewSemVer(version.Version)
	if err != nil {
		return "", fmt.Errorf("error creating version: %v", err)
	}
	cm.ACVersion = *v

	for _, img := range cfg.Images {
		am, err := setupAppImage(cfg, img, dir)
		if err != nil {
			return "", fmt.Errorf("error setting up image %s: %v", img, err)
		}
		if cm.Apps.Get(am.Name) != nil {
			return "", fmt.Errorf("error: multiple apps with name %s", am.Name)
		}
		if am.App == nil {
			return "", fmt.Errorf("error: image %s has no app section", img)
		}
		a := schema.RuntimeApp{
			Name:        am.Name,
			ImageID:     img,
			Isolators:   am.App.Isolators,
			Annotations: am.Annotations,
		}
		cm.Apps = append(cm.Apps, a)
	}

	// TODO(jonboulle): check that app mountpoint expectations are
	// satisfied here, rather than waiting for stage1
	cm.Volumes = cfg.Volumes

	cdoc, err := json.Marshal(cm)
	if err != nil {
		return "", fmt.Errorf("error marshalling container manifest: %v", err)
	}

	log.Printf("Writing container manifest")
	fn := common.ContainerManifestPath(dir)
	if err := ioutil.WriteFile(fn, cdoc, 0700); err != nil {
		return "", fmt.Errorf("error writing container manifest: %v", err)
	}
	return dir, nil
}

// Run actually runs the container by exec()ing the stage1 init inside
// the container filesystem.
func Run(cfg Config, dir string) {
	if cfg.SpawnMetadataSvc {
		log.Print("Launching metadata svc")
		if err := launchMetadataSvc(cfg.Debug); err != nil {
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
	if err := syscall.Exec(args[0], args, os.Environ()); err != nil {
		log.Fatalf("error execing init: %v", err)
	}
}

// makeUniqueContainer creates a subdirectory (representing a container)
// within the given parent directory. On success, it returns a UUID
// representing the created container and the full path to the new directory.
// The UUID is guaranteed to be unique within the parent directory.
// The parent directory MUST exist and be writeable.
func makeUniqueContainer(pdir string) (*types.UUID, string, error) {
	// Arbitrary limit so we don't spin forever
	for i := 0; i <= 100; i++ {
		cuuid, err := types.NewUUID(uuid.New())
		if err != nil {
			// Should never happen
			return nil, "", fmt.Errorf("error creating UUID: %v", err)
		}

		dir := filepath.Join(pdir, cuuid.String())
		err = os.Mkdir(dir, 0700)
		switch {
		case err == nil:
			return cuuid, dir, nil
		case os.IsExist(err):
			continue
		case err != nil:
			return nil, "", err
		}
	}
	return nil, "", fmt.Errorf("couldn't find unique directory!")
}

func lockDir(dir string) error {
	l, err := lock.TryExclusiveDirLock(dir)
	if err != nil {
		return fmt.Errorf("error acquiring lock on dir %q: %v", dir, err)
	}
	// We need the fd number for stage1 and leave the file open / lock held til process exit
	fd, err := l.Fd()
	if err != nil {
		panic(err)
	}
	return os.Setenv(envLockFd, fmt.Sprintf("%v", fd))
}

// setupAppImage attempts to load the app image by the given hash from the store,
// verifies that the image matches the hash, and extracts the image into a
// directory in the given dir.
// It returns the ImageManifest that the image contains.
// TODO(jonboulle): tighten up the Hash type here; currently it is partially-populated (i.e. half-length sha512)
func setupAppImage(cfg Config, img types.Hash, cdir string) (*schema.ImageManifest, error) {
	log.Println("Loading image", img.String())

	ad := common.AppImagePath(cdir, img)
	err := os.MkdirAll(ad, 0776)
	if err != nil {
		return nil, fmt.Errorf("error creating image directory: %v", err)
	}

	if err := expandImage(cfg, img, ad); err != nil {
		return nil, fmt.Errorf("error expanding app image: %v", err)
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
func setupStage1Image(cfg Config, img types.Hash, cdir string) error {
	s1 := common.Stage1ImagePath(cdir)
	if err := os.MkdirAll(s1, 0755); err != nil {
		return fmt.Errorf("error creating stage1 directory: %v", err)
	}
	if err := expandImage(cfg, img, s1); err != nil {
		return fmt.Errorf("error expanding stage1 image: %v", err)
	}
	return nil
}

// expandImage attempts to load the image by the given hash from the store,
// verifies that the image matches the hash, and extracts the image at the specified destination.
func expandImage(cfg Config, img types.Hash, dest string) error {
	rs, err := cfg.Store.ReadStream(img.String())
	if err != nil {
		return fmt.Errorf("error reading stream: %v", err)
	}

	hash := sha512.New()
	r := io.TeeReader(rs, hash)

	if err := ptar.ExtractTar(tar.NewReader(r), dest, false, nil); err != nil {
		return fmt.Errorf("error extracting ACI: %v", err)
	}

	// Tar does not necessarily read the complete file, so ensure we read the entirety into the hash
	if _, err := io.Copy(ioutil.Discard, r); err != nil {
		return fmt.Errorf("error reading ACI: %v", err)
	}

	// TODO(jonboulle): clean this up, leaky abstraction with the store.
	if g := cas.HashToKey(hash); g != img.String() {
		return fmt.Errorf("image hash does not match expected (%v != %v)", g, img.String())
	}

	return nil
}

func launchMetadataSvc(debug bool) error {
	// use socket activation protocol to avoid race-condition of
	// service becoming ready
	l, err := net.ListenTCP("tcp4", &net.TCPAddr{Port: common.MetadataSvcPrvPort})
	if err != nil {
		if err.(*net.OpError).Err.(*os.SyscallError).Err == syscall.EADDRINUSE {
			// assume metadatasvc is already running
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
	args = append(args, "metadatasvc", "--no-idle")

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
