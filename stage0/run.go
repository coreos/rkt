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
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"crypto/sha512"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"syscall"

	"github.com/appc/spec/aci"
	"github.com/appc/spec/schema"
	"github.com/appc/spec/schema/types"
	"github.com/coreos/rocket/Godeps/_workspace/src/code.google.com/p/go-uuid/uuid"
	"github.com/coreos/rocket/cas"
	"github.com/coreos/rocket/common"
	"github.com/coreos/rocket/pkg/lock"
	ptar "github.com/coreos/rocket/pkg/tar"
	"github.com/coreos/rocket/version"

	"github.com/coreos/rocket/stage0/stage1_init"
	"github.com/coreos/rocket/stage0/stage1_rootfs"
)

const (
	initPath  = "stage1/init"
	envLockFd = "RKT_LOCK_FD"
)

type Config struct {
	Store         *cas.Store // store containing all of the configured application images
	ContainersDir string     // root directory for rocket containers
	Stage1Init    string     // binary to be execed as stage1
	Stage1Rootfs  string     // compressed bundle containing a rootfs for stage1
	Debug         bool
	// TODO(jonboulle): These images are partially-populated hashes, this should be clarified.
	Images           []types.Hash      // application images
	Volumes          map[string]string // map of volumes that rocket can provide to applications
	PrivateNet       bool              // container should have its own network stack
	SpawnMetadataSvc bool              // launch metadata service
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

	log.Printf("Unpacking stage1 rootfs")
	if cfg.Stage1Rootfs != "" {
		err = unpackRootfs(cfg.Stage1Rootfs, common.Stage1RootfsPath(dir))
	} else {
		err = unpackBuiltinRootfs(common.Stage1RootfsPath(dir))
	}
	if err != nil {
		return "", fmt.Errorf("error unpacking rootfs: %v", err)
	}

	log.Printf("Writing stage1 init")
	var in io.Reader
	if cfg.Stage1Init != "" {
		in, err = os.Open(cfg.Stage1Init)
		if err != nil {
			return "", fmt.Errorf("error loading stage1 init binary: %v", err)
		}
	} else {
		init_bin, err := stage1_init.Asset("s1init")
		if err != nil {
			return "", fmt.Errorf("error accessing stage1 init bindata: %v", err)
		}
		in = bytes.NewBuffer(init_bin)
	}
	fn := filepath.Join(dir, initPath)
	out, err := os.OpenFile(fn, os.O_CREATE|os.O_WRONLY, 0555)
	if err != nil {
		return "", fmt.Errorf("error opening stage1 init for writing: %v", err)
	}
	if _, err := io.Copy(out, in); err != nil {
		return "", fmt.Errorf("error writing stage1 init: %v", err)
	}
	if err := out.Close(); err != nil {
		return "", fmt.Errorf("error closing stage1 init: %v", err)
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
		am, err := setupImage(cfg, img, dir)
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

	var sVols []types.Volume
	for key, path := range cfg.Volumes {
		v := types.Volume{
			Kind:     "host",
			Source:   path,
			ReadOnly: true,
			Fulfills: []types.ACName{
				types.ACName(key),
			},
		}
		sVols = append(sVols, v)
	}
	// TODO(jonboulle): check that app mountpoint expectations are
	// satisfied here, rather than waiting for stage1
	cm.Volumes = sVols

	cdoc, err := json.Marshal(cm)
	if err != nil {
		return "", fmt.Errorf("error marshalling container manifest: %v", err)
	}

	log.Printf("Writing container manifest")
	fn = common.ContainerManifestPath(dir)
	if err := ioutil.WriteFile(fn, cdoc, 0700); err != nil {
		return "", fmt.Errorf("error writing container manifest: %v", err)
	}
	return dir, nil
}

// Run actually runs the container by exec()ing the stage1 init inside
// the container filesystem.
func Run(cfg Config, dir string) {
	log.Printf("Pivoting to filesystem %s", dir)
	if err := os.Chdir(dir); err != nil {
		log.Fatalf("failed changing to dir: %v", err)
	}

	log.Printf("Execing %s", initPath)
	args := []string{initPath}
	if cfg.Debug {
		args = append(args, "--debug")
	}
	if cfg.SpawnMetadataSvc {
		rktExe, err := os.Readlink("/proc/self/exe")
		if err != nil {
			log.Fatalf("failed to readlink /proc/self/exe: %v", err)
		}
		dbgFlag := ""
		if cfg.Debug {
			dbgFlag = " --debug"
		}
		args = append(args, fmt.Sprintf("--metadata-svc=%s%s metadatasvc --no-idle", rktExe, dbgFlag))
	}
	if cfg.PrivateNet {
		args = append(args, "--private-net")
	}
	if err := syscall.Exec(initPath, args, os.Environ()); err != nil {
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
	l, err := lock.TryExclusiveLock(dir)
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

func untarRootfs(r io.Reader, dir string) error {
	tr := tar.NewReader(r)
	if err := os.MkdirAll(dir, 0776); err != nil {
		return fmt.Errorf("error creating stage1 rootfs directory: %v", err)
	}

	if err := ptar.ExtractTar(tr, dir, false, nil); err != nil {
		return fmt.Errorf("error extracting rootfs: %v", err)
	}
	return nil
}

// unpackRootfs unpacks a stage1 rootfs (compressed file, pointed to by rfs)
// into dir, returning any error encountered
func unpackRootfs(rfs string, dir string) error {
	fh, err := os.Open(rfs)
	if err != nil {
		return fmt.Errorf("error opening stage1 rootfs: %v", err)
	}
	typ, err := aci.DetectFileType(fh)
	if err != nil {
		return fmt.Errorf("error detecting image type: %v", err)
	}
	if _, err := fh.Seek(0, 0); err != nil {
		return fmt.Errorf("error seeking image: %v", err)
	}
	var r io.Reader
	switch typ {
	case aci.TypeGzip:
		r, err = gzip.NewReader(fh)
		if err != nil {
			return fmt.Errorf("error reading gzip: %v", err)
		}
	case aci.TypeBzip2:
		r = bzip2.NewReader(fh)
	case aci.TypeXz:
		r = aci.XzReader(fh)
	case aci.TypeTar:
		r = fh
	case aci.TypeUnknown:
		return fmt.Errorf("error: unknown image filetype")
	default:
		// should never happen
		panic("no type returned from DetectFileType?")
	}
	return untarRootfs(r, dir)
}

// unpackBuiltinRootfs unpacks the included stage1 rootfs into dir
func unpackBuiltinRootfs(dir string) error {
	b, err := stage1_rootfs.Asset("s1rootfs.tar")
	if err != nil {
		return fmt.Errorf("error accessing rootfs asset: %v", err)
	}
	buf := bytes.NewBuffer(b)
	return untarRootfs(buf, dir)
}

// setupImage attempts to load the image by the given hash from the store,
// verifies that the image matches the hash, and extracts the image into a
// directory in the given dir.
// It returns the ImageManifest that the image contains.
// TODO(jonboulle): tighten up the Hash type here; currently it is partially-populated (i.e. half-length sha512)
func setupImage(cfg Config, img types.Hash, dir string) (*schema.ImageManifest, error) {
	log.Println("Loading image", img.String())

	rs, err := cfg.Store.ReadStream(img.String())
	if err != nil {
		return nil, fmt.Errorf("error reading stream: %v", err)
	}

	ad := common.AppImagePath(dir, img)
	err = os.MkdirAll(ad, 0776)
	if err != nil {
		return nil, fmt.Errorf("error creating image directory: %v", err)
	}

	hash := sha512.New()
	r := io.TeeReader(rs, hash)

	if err := ptar.ExtractTar(tar.NewReader(r), ad, false, nil); err != nil {
		return nil, fmt.Errorf("error extracting ACI: %v", err)
	}

	// Tar does not necessarily read the complete file, so ensure we read the entirety into the hash
	if _, err := io.Copy(ioutil.Discard, r); err != nil {
		return nil, fmt.Errorf("error reading ACI: %v", err)
	}

	// TODO(jonboulle): clean this up, leaky abstraction with the store.
	if g := cas.HashToKey(hash); g != img.String() {
		if err := os.RemoveAll(ad); err != nil {
			fmt.Fprintf(os.Stderr, "error cleaning up directory: %v\n", err)
		}
		return nil, fmt.Errorf("image hash does not match expected (%v != %v)", g, img.String())
	}

	err = os.MkdirAll(filepath.Join(ad, "rootfs/tmp"), 0777)
	if err != nil {
		return nil, fmt.Errorf("error creating tmp directory: %v", err)
	}

	mpath := common.ImageManifestPath(dir, img)
	f, err := os.Open(mpath)
	if err != nil {
		return nil, fmt.Errorf("error opening app manifest: %v", err)
	}
	b, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("error reading app manifest: %v", err)
	}
	var am schema.ImageManifest
	if err := json.Unmarshal(b, &am); err != nil {
		return nil, fmt.Errorf("error unmarshaling app manifest: %v", err)
	}

	// Ensure the device files are created
	for _, v := range am.DeviceFiles {
		log.Println("Device files: ", v)
		fi, err := os.Stat(v)
		if err != nil {
			return nil, fmt.Errorf("Cannot stat %s: %v", v, err)
		}
		if fi.Mode() & os.ModeType != os.ModeCharDevice && fi.Mode() & os.ModeType != os.ModeDevice {
			return nil, fmt.Errorf("File %s is not a device file", v)
		}
		st, ok := fi.Sys().(*syscall.Stat_t)
		if !ok {
			return nil, fmt.Errorf("Cannot stat %s", v)
		}

		p := filepath.Join(ad, "rootfs", v)

		if err := syscall.Mknod(p, st.Mode, int(st.Rdev)); err != nil {
			return nil, fmt.Errorf("error creating device file %s: %v", v, err)
		}
	}

	return &am, nil
}
