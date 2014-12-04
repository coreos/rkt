package main

import (
	"archive/tar"
	"compress/gzip"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/coreos/rocket/app-container/aci"
	"github.com/coreos/rocket/app-container/schema"
	"github.com/coreos/rocket/pkg/tarheader"
)

var (
	buildFilesetName     string
	buildAppManifest     string
	buildFilesetManifest string
	buildRootfs          bool
	buildOverwrite       bool
	cmdBuild             = &Command{
		Name:        "build",
		Description: "Build a Fileset ACI from the target directory",
		Summary:     "Build a Fileset ACI from the target directory",
		Usage:       "[--overwrite] --name=NAME DIRECTORY OUTPUT_FILE",
		Run:         runBuild,
	}
)

func init() {
	cmdBuild.Flags.StringVar(&buildFilesetName, "fileset-name", "",
		"Build a Fileset Image, by this name (e.g. example.com/reduce-worker)")
	cmdBuild.Flags.StringVar(&buildAppManifest, "app-manifest", "",
		"Build an App Image with this App Manifest")
	cmdBuild.Flags.StringVar(&buildFilesetManifest, "fileset-manifest", "",
		"Build an App Image with this Fileset Manifest (optional)")
	cmdBuild.Flags.BoolVar(&buildRootfs, "rootfs", true,
		"Whether the supplied directory is a rootfs. If false, it will be assume the supplied directory already contains a rootfs/ subdirectory.")
	cmdBuild.Flags.BoolVar(&buildOverwrite, "overwrite", false, "Overwrite target file if it already exists")
}

func buildWalker(root string, aw aci.ArchiveWriter, rootfs bool) filepath.WalkFunc {
	return func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relpath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rootfs {
			if relpath == "." {
				relpath = ""
			}
			relpath = "rootfs/" + relpath
		}
		if relpath == "." {
			return nil
		}

		link := ""
		var file *os.File
		switch info.Mode() & os.ModeType {
		default:
			file, err = os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()
		case os.ModeSymlink:
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			link = target
		}

		hdr, err := tar.FileInfoHeader(info, link)
		if err != nil {
			panic(err)
		}
		// Because os.FileInfo's Name method returns only the base
		// name of the file it describes, it may be necessary to
		// modify the Name field of the returned header to provide the
		// full path name of the file.
		hdr.Name = relpath
		tarheader.Populate(hdr, info)
		aw.AddFile(relpath, hdr, file)

		return nil
	}
}

func runBuild(args []string) (exit int) {
	if len(args) != 2 {
		stderr("build: Must provide directory and output file")
		return 1
	}
	switch {
	case buildFilesetName != "" && buildAppManifest == "":
	case buildFilesetName == "" && buildAppManifest != "":
	default:
		stderr("build: must specify either --fileset-name or --app-manifest")
		return 1
	}

	root := args[0]
	tgt := args[1]
	ext := filepath.Ext(tgt)
	if ext != schema.ACIExtension {
		stderr("build: Extension must be %s (given %s)", schema.ACIExtension, ext)
		return 1
	}

	mode := os.O_CREATE | os.O_WRONLY
	if !buildOverwrite {
		mode |= os.O_EXCL
	}
	fh, err := os.OpenFile(tgt, mode, 0655)
	if err != nil {
		if os.IsExist(err) {
			stderr("build: Target file exists (try --overwrite)")
		} else {
			stderr("build: Unable to open target %s: %v", tgt, err)
		}
		return 1
	}

	gw := gzip.NewWriter(fh)
	tr := tar.NewWriter(gw)

	defer func() {
		tr.Close()
		gw.Close()
		fh.Close()
		if exit != 0 && !buildOverwrite {
			os.Remove(tgt)
		}
	}()

	var aw aci.ArchiveWriter
	if buildFilesetName != "" {
		fsm, err := schema.NewFilesetManifest(buildFilesetName)
		if err != nil {
			stderr("build: Unable to create Fileset Manifest: %v", err)
			return 1
		}
		aw = aci.NewAppWriter(nil, fsm, tr)
	} else {
		b, err := ioutil.ReadFile(buildAppManifest)
		if err != nil {
			stderr("build: Unable to read App Manifest: %v", err)
			return 1
		}
		var am schema.AppManifest
		if err := am.UnmarshalJSON(b); err != nil {
			stderr("build: Unable to load App Manifest: %v", err)
			return 1
		}

		if buildFilesetManifest != "" {
			//TODO(elcct): clean this up
			fsm, err := schema.NewFilesetManifest(buildFilesetManifest)
			if err != nil {
				stderr("build: Unable to create Fileset Manifest: %v", err)
				return 1
			}
			b, err = ioutil.ReadFile(buildFilesetManifest)
			if err := fsm.UnmarshalJSON(b); err != nil {
				stderr("build: Unable to load Fileset Manifest: %v", err)
				return 1
			}
			aw = aci.NewAppWriter(&am, fsm, tr)
		} else {
			aw = aci.NewAppWriter(&am, nil, tr)
		}

	}

	err = filepath.Walk(root, buildWalker(root, aw, buildRootfs))
	if err != nil {
		stderr("build: Error walking rootfs: %v", err)
		return 1
	}

	err = aw.Close()
	if err != nil {
		stderr("build: Unable to close Fileset image %s: %v", tgt, err)
		return 1
	}

	return
}
