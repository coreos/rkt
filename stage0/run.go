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
	"crypto/sha256"
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
	rktpath "github.com/coreos/rocket/path"
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
	Images        []types.Hash      // application images
	Volumes       map[string]string // map of volumes that rocket can provide to applications
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

	cuuid, err := types.NewUUID(uuid.New())
	if err != nil {
		return "", fmt.Errorf("error creating UID: %v", err)
	}

	// TODO(jonboulle): collision detection/mitigation
	// Create a directory for this container
	dir := filepath.Join(cfg.ContainersDir, cuuid.String())

	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("error creating directory: %v", err)
	}

	// Set up the container lock
	l, err := lock.NewLock(dir)
	if err == nil {
		err = l.ExclusiveLock()
	}
	if err != nil {
		return "", fmt.Errorf("error locking directory: %v", err)
	}

	log.Printf("Unpacking stage1 rootfs")
	if cfg.Stage1Rootfs != "" {
		err = unpackRootfs(cfg.Stage1Rootfs, rktpath.Stage1RootfsPath(dir))
	} else {
		err = unpackBuiltinRootfs(rktpath.Stage1RootfsPath(dir))
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
	fn = rktpath.ContainerManifestPath(dir)
	if err := ioutil.WriteFile(fn, cdoc, 0700); err != nil {
		return "", fmt.Errorf("error writing container manifest: %v", err)
	}
	return dir, nil
}

// Run actually runs the container by exec()ing the stage1 init inside
// the container filesystem.
func Run(dir string, debug bool) {
	log.Printf("Pivoting to filesystem %s", dir)
	if err := os.Chdir(dir); err != nil {
		log.Fatalf("failed changing to dir: %v", err)
	}

	log.Printf("Execing %s", initPath)
	args := []string{initPath}
	if debug {
		args = append(args, "debug")
	}
	if err := syscall.Exec(initPath, args, os.Environ()); err != nil {
		log.Fatalf("error execing init: %v", err)
	}
}

func lockDir(dir string) error {
	fd, err := syscall.Open(dir, syscall.O_DIRECTORY, 0)
	if err != nil {
		return fmt.Errorf("error opening directory: %v", err)
	}

	if err := syscall.Flock(fd, syscall.LOCK_NB|syscall.LOCK_EX); err != nil {
		return fmt.Errorf("error acquiring lock on dir %q: %v", dir, err)
	}

	return os.Setenv(envLockFd, fmt.Sprintf("%v", fd))
}

func untarRootfs(r io.Reader, dir string) error {
	tr := tar.NewReader(r)
	if err := os.MkdirAll(dir, 0776); err != nil {
		return fmt.Errorf("error creating stage1 rootfs directory: %v", err)
	}

	if err := ptar.ExtractTar(tr, dir); err != nil {
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
// verifies that the image matches the given hash and extracts the image
// into a directory in the given dir.
// It returns the ImageManifest that the image contains
func setupImage(cfg Config, img types.Hash, dir string) (*schema.ImageManifest, error) {
	log.Println("Loading image", img.String())

	rs, err := cfg.Store.ReadStream(img.String())
	if err != nil {
		return nil, err
	}

	ad := rktpath.AppImagePath(dir, img)
	err = os.MkdirAll(ad, 0776)
	if err != nil {
		return nil, fmt.Errorf("error creating image directory: %v", err)
	}

	hash := sha256.New()
	r := io.TeeReader(rs, hash)

	if err := ptar.ExtractTar(tar.NewReader(r), ad); err != nil {
		return nil, fmt.Errorf("error extracting ACI: %v", err)
	}

	// Tar does not necessarily read the complete file, so ensure we read the entirety into the hash
	if _, err := io.Copy(ioutil.Discard, r); err != nil {
		return nil, fmt.Errorf("error reading ACI: %v", err)
	}

	if id := fmt.Sprintf("%x", hash.Sum(nil)); id != img.Val {
		if err := os.RemoveAll(ad); err != nil {
			fmt.Fprintf(os.Stderr, "error cleaning up directory: %v\n", err)
		}
		return nil, fmt.Errorf("image hash does not match expected (%v != %v)", id, img.Val)
	}

	err = os.MkdirAll(filepath.Join(ad, "rootfs/tmp"), 0777)
	if err != nil {
		return nil, fmt.Errorf("error creating tmp directory: %v", err)
	}

	mpath := rktpath.ImageManifestPath(dir, img)
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
	return &am, nil
}
