package main

import (
	"archive/tar"
	"compress/bzip2"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/coreos-inc/rkt/app-container/aci"
	"github.com/coreos-inc/rkt/app-container/fileset"
	"github.com/coreos-inc/rkt/app-container/schema"
)

const (
	typeAppImage = "appimage"
	typeManifest = "manifest"
)

var (
	valType     string
	cmdValidate = &Command{
		Name:        "validate",
		Description: "Validate an AppContainer file",
		Summary:     "Validate an AppContainer file",
		Run:         runValidate,
	}
	types = []string{
		typeAppImage,
		typeManifest,
	}
)

func init() {
	cmdValidate.Flags.StringVar(&valType, "type", "",
		fmt.Sprintf(`Type of file to validate. If unset, actool will try to detect the type. One of "%s"`, strings.Join(types, ",")))
}

func runValidate(args []string) (exit int) {
	if len(args) < 1 {
		stderr("must pass one or more files")
		return 1
	}

	for _, path := range args {
		file, err := os.OpenFile(path, os.O_RDONLY, 0666)
		if err != nil {
			stderr("unable to open %s: %v", path, err)
			return 1
		}

		if valType == "" {
			valType, err = detectValType(file)
			if err != nil {
				stderr("error detecting file type: %v", err)
				return 1
			}
		}
		switch valType {
		case typeAppImage:
			tr := tar.NewReader(file)
			err := fileset.ValidateArchive(tr)
			//			err := validateACI(file)
			file.Close()
			if err != nil {
				stderr("%s: error validating: %v", path, err)
				return 1
			}
			if globalFlags.Debug {
				stderr("%s: valid fileset", path)
			}
			continue
		case typeManifest:
			b, err := ioutil.ReadAll(file)
			file.Close()
			if err != nil {
				stderr("%s: unable to read file %s", path, err)
				return 1
			}
			k := schema.Kind{}
			if err := k.UnmarshalJSON(b); err != nil {
				stderr("error unmarshaling manifest: %v", err)
				return 1
			}
			switch k.ACKind {
			case "AppManifest":
				m := schema.AppManifest{}
				err = m.UnmarshalJSON(b)
			case "ContainerRuntimeManifest":
				m := schema.ContainerRuntimeManifest{}
				err = m.UnmarshalJSON(b)
			case "FileSetManifest":
				m := schema.FileSetManifest{}
				err = m.UnmarshalJSON(b)
			default:
				// Should not get here; schema.Kind unmarshal should fail
				panic("bad ACKind")
			}
			if err != nil {
				stderr("%s: invalid %s: %v", path, k.ACKind, err)
			} else if globalFlags.Debug {
				stderr("%s: valid %s", path, k.ACKind)
			}
		default:
			stderr("%s: unable to detect filetype (try --type)", path)
			return 1
		}
	}

	return
}

func detectValType(file *os.File) (string, error) {
	typ, err := aci.DetectFileType(file)
	if err != nil {
		return "", err
	}
	if _, err := file.Seek(0, 0); err != nil {
		return "", err
	}
	switch typ {
	case aci.TypeXz, aci.TypeGzip, aci.TypeBzip2, aci.TypeTar:
		return typeAppImage, nil
	case aci.TypeText:
		return typeManifest, nil
	default:
		return "", nil
	}
}

func validateACI(rs io.ReadSeeker) error {
	// TODO(jonboulle): this is a bit redundant with detectValType
	typ, err := aci.DetectFileType(rs)
	if err != nil {
		return err
	}
	if _, err := rs.Seek(0, 0); err != nil {
		return err
	}
	var r io.Reader
	switch typ {
	case aci.TypeGzip:
		r, err = gzip.NewReader(rs)
		if err != nil {
			return fmt.Errorf("error reading gzip: %v", err)
		}
	case aci.TypeBzip2:
		r = bzip2.NewReader(rs)
	case aci.TypeXz:
		r = aci.XzReader(rs)
	case aci.TypeUnknown:
		return errors.New("unknown filetype")
	default:
		// should never happen
		panic(fmt.Sprintf("bad type returned from DetectFileType: %v", typ))
	}
	return aci.ValidateTar(r)
}
