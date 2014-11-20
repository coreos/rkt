package taf

// Package taf contains a small library to validate files that comply with the TAF spec

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"golang.org/x/crypto/openpgp"
	"github.com/containers/standard/raf"
)

// TODO(jonboulle): support detached signatures

// LoadSignedData reads PGP encrypted data from the given Reader, using the
// provided keyring (EntityList). The entire decrypted bytestream is
// returned, and/or any error encountered.
// TODO(jonboulle): support symmetric decryption
func LoadSignedData(signed io.Reader, kr openpgp.EntityList) ([]byte, error) {
	md, err := openpgp.ReadMessage(signed, kr, nil, nil)
	if err != nil {
		return nil, err
	}
	if md.IsSymmetricallyEncrypted {
		return nil, errors.New("symmetric encryption not yet supported")
	}

	// Signature cannot be verified until body is read
	data, err := ioutil.ReadAll(md.UnverifiedBody)
	if err != nil {
		return nil, fmt.Errorf("error reading body: %v", err)
	}
	if md.IsSigned && md.SignedBy != nil {
		// Once EOF has been seen, the following fields are
		// valid. (An authentication code failure is reported as a
		// SignatureError error when reading from UnverifiedBody.)
		//
		if md.SignatureError != nil {
			return nil, fmt.Errorf("signature error: %v", md.SignatureError)
		}
		log.Println("message signature OK")
	}
	return data, nil
}

// ValidateTarball checks that a bundle o'bytes is a valid GZIP-compressed
// TAR file that contains a directory layout which matches the RAF spec
func ValidateTarball(data []byte) error {
	// TODO(jonboulle): do this in memory instead of writing out to disk? :/
	dir, err := ioutil.TempDir("", "taf-validator")
	if err != nil {
		return fmt.Errorf("error creating tempdir for RAF validation: %v", err)
	}
	defer os.RemoveAll(dir)
	r := bytes.NewReader(data)
	gz, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("error reading tarball: %v", err)
	}
	t := tar.NewReader(gz)
	err = ExtractTar(t, dir)
	if err != nil {
		return err
	}
	return raf.ValidateRAF(dir)
}

// ExtractTar extracts a tarball (from a tar.Reader) into the given directory
func ExtractTar(tr *tar.Reader, dir string) error {
	for {
		hdr, err := tr.Next()
		switch err {
		case io.EOF:
			return nil
		case nil:
			p := filepath.Join(dir, hdr.Name)
			fi := hdr.FileInfo()
			typ := hdr.Typeflag
			switch {
			case typ == tar.TypeReg || typ == tar.TypeRegA:
				f, err := os.OpenFile(p, os.O_CREATE|os.O_RDWR, fi.Mode())
				if err != nil {
					return err
				}
				_, err = io.Copy(f, tr)
				if err != nil {
					return err
				}
				f.Close()
			case typ == tar.TypeDir:
				if err := os.MkdirAll(p, fi.Mode()); err != nil {
					return err
				}
			case typ == tar.TypeLink:
				if err := os.Link(hdr.Linkname, p); err != nil {
					return err
				}
			case typ == tar.TypeSymlink:
				if err := os.Symlink(hdr.Linkname, p); err != nil {
					return err
				}
			// TODO(jonboulle): implement other modes
			default:
				return fmt.Errorf("unsupported type: %v", typ)
			}
		default:
			return fmt.Errorf("error extracting tarball: %v", err)
		}
	}
}
