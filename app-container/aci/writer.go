package aci

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"io"
	"time"

	"github.com/coreos/rocket/app-container/schema"
)

// ArchiveWriter writes App Container Images. Users wanting to create an ACI or
// should create an ArchiveWriter and add files to it; the ACI will be written
// to the underlying tar.Writer
type ArchiveWriter interface {
	AddFile(path string, hdr *tar.Header, r io.Reader) error
	Close() error
}

type appArchiveWriter struct {
	*tar.Writer
	am *schema.ImageManifest
}

// NewAppWriter creates a new ArchiveWriter which will generate an App
// Container Image based on the given manifest and write it to the given
// tar.Writer
func NewAppWriter(am schema.ImageManifest, w *tar.Writer) ArchiveWriter {
	aw := &appArchiveWriter{
		w,
		&am,
	}
	return aw
}

func (aw *appArchiveWriter) AddFile(path string, hdr *tar.Header, r io.Reader) error {
	err := aw.Writer.WriteHeader(hdr)
	if err != nil {
		return err
	}

	if r != nil {
		_, err := io.Copy(aw.Writer, r)
		if err != nil {
			return err
		}
	}

	return nil
}

func (aw *appArchiveWriter) addFileNow(path string, contents []byte) error {
	buf := bytes.NewBuffer(contents)
	now := time.Now()
	hdr := tar.Header{
		Name:       path,
		Mode:       0644,
		Uid:        0,
		Gid:        0,
		Size:       int64(buf.Len()),
		ModTime:    now,
		Typeflag:   tar.TypeReg,
		Uname:      "root",
		Gname:      "root",
		ChangeTime: now,
	}
	return aw.AddFile(path, &hdr, buf)
}

func (aw *appArchiveWriter) addManifest(name string, m json.Marshaler) error {
	out, err := m.MarshalJSON()
	if err != nil {
		return err
	}
	return aw.addFileNow(name, out)
}

func (aw *appArchiveWriter) Close() error {
	if err := aw.addManifest("app", aw.am); err != nil {
		return err
	}
	return aw.Writer.Close()
}
