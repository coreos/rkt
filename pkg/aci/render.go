package aci

import (
	"archive/tar"
	"fmt"
	"os"
	"runtime"
	"syscall"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/pkg/acirenderer"
	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/schema/types"
	ptar "github.com/coreos/rkt/pkg/tar"
)

// Given an imageID, start with the matching image available in the store,
// build its dependency list and render it inside dir
func RenderACIWithImageID(imageID types.Hash, dir string, ap acirenderer.ACIRegistry) error {
	renderedACI, err := acirenderer.GetRenderedACIWithImageID(imageID, ap)
	if err != nil {
		return err
	}
	return renderImage(renderedACI, dir, ap)
}

// Given an image app name and optional labels, get the best matching image
// available in the store, build its dependency list and render it inside dir
func RenderACI(name types.ACName, labels types.Labels, dir string, ap acirenderer.ACIRegistry) error {
	renderedACI, err := acirenderer.GetRenderedACI(name, labels, ap)
	if err != nil {
		return err
	}
	return renderImage(renderedACI, dir, ap)
}

// Given an already populated dependency list, it will extract, under the provided
// directory, the rendered ACI
func RenderACIFromList(imgs acirenderer.Images, dir string, ap acirenderer.ACIProvider) error {
	renderedACI, err := acirenderer.GetRenderedACIFromList(imgs, ap)
	if err != nil {
		return err
	}
	return renderImage(renderedACI, dir, ap)
}

// Given a RenderedACI, it will extract, under the provided directory, the
// needed files from the right source ACI.
// The manifest will be extracted from the upper ACI.
// No file overwriting is done as it should usually be called
// providing an empty directory.
func renderImage(renderedACI acirenderer.RenderedACI, dir string, ap acirenderer.ACIProvider) error {
	for _, ra := range renderedACI {
		rs, err := ap.ReadStream(ra.Key)
		if err != nil {
			return err
		}
		defer rs.Close()

		c := make(chan error)
		go func() {
			// Perform extraction in a chroot.
			// We can't trust the tar to not recur at symlinks, and we want to be able to recreate symlinks verbatim.
			// Here we burn an OS thread on the extraction and unshare CLONE_FS to allow a thread-local chroot().
			// It's this or fork & exec to do the chroot, inconvenient and would require more IPC.
			runtime.LockOSThread()

			// To prevent the scheduler from creating a new OS thread for the calling goroutine at some point post-unshare, force schedule now.
			runtime.Gosched()

			// XXX(vc): If more headroom is needed to prevent blocking operations in ExtractTar() from triggering a clone(), we can cause more
			// threads to be created here.

			// FIXME(vc): Go needs to add something like runtime.TaintOSThread() which we would call before Unshare()
			// This could be used to prevent the scheduler from calling clone() directly in this thread, delegating the create to a manager thread.

			err = syscall.Unshare(int(0x00000200)) // CLONE_FS
			if err != nil {
				c <- fmt.Errorf("error unsharing CLONE_FS: %v", err)
			}

			if err == nil {
				err = os.MkdirAll(dir, 0755)
				if err != nil {
					c <- fmt.Errorf("error creating dest dir: %v", err)
				}
			}

			if err == nil {
				err = syscall.Chroot(dir)
				if err != nil {
					c <- fmt.Errorf("error chrooting to %s: %v", dir, err)
				}
			}

			if err == nil {
				err = os.Chdir("/")
				if err != nil {
					c <- fmt.Errorf("error chdiring to /: %v", err)
				}
			}

			if err == nil {
				// Overwrite is not needed. If a file needs to be overwritten then the renderedACI builder has a bug
				err = ptar.ExtractTar(tar.NewReader(rs), "/", false, ra.FileMap)
				if err != nil {
					c <- fmt.Errorf("error extracting ACI: %v", err)
				}
			}

			if err == nil {
				c <- nil
			}

			// FIXME(vc): We need a way in Go to force the current OS thread to be discarded, for when we've done irreversible things like Unshare() to it.
			// For now we just occopy it indefinitely while locked, which achieves the same thing, just wastes an OS thread, harmless in rkt.
			// If there were something like runtime.TaintOSThread() it could automatically do this on our behalf when locked on runtime.Goexit()

			// XXX(vc): This causes z_last_test.go to fail.
			select {}
		}()
		return <-c
	}

	return nil
}
