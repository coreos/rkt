package tar

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

const DEFAULT_DIR_MODE os.FileMode = 0755

type insecureLinkError error

// ExtractTar extracts a tarball (from a tar.Reader) into the given directory
func ExtractTar(tr *tar.Reader, dir string) error {
	um := syscall.Umask(0)
	defer syscall.Umask(um)
	for {
		hdr, err := tr.Next()
		switch err {
		case io.EOF:
			return nil
		case nil:
			p := filepath.Join(dir, hdr.Name)
			fi := hdr.FileInfo()
			typ := hdr.Typeflag

			// Create parent dir if it doesn't exists
			if err := os.MkdirAll(filepath.Dir(p), DEFAULT_DIR_MODE); err != nil {
				return err
			}

			switch {
			case typ == tar.TypeReg || typ == tar.TypeRegA:
				f, err := os.OpenFile(p, os.O_CREATE|os.O_RDWR, fi.Mode())
				if err != nil {
					f.Close()
					return err
				}
				_, err = io.Copy(f, tr)
				if err != nil {
					f.Close()
					return err
				}
				f.Close()
			case typ == tar.TypeDir:
				if err := os.MkdirAll(p, fi.Mode()); err != nil {
					return err
				}
				dir, err := os.Open(p)
				if err != nil {
					return err
				}
				if err := dir.Chmod(fi.Mode()); err != nil {
					return err
				}
			case typ == tar.TypeLink:
				dest := filepath.Join(dir, hdr.Linkname)
				if !strings.HasPrefix(dest, dir) {
					return insecureLinkError(fmt.Errorf("insecure link %q -> %q", p, hdr.Linkname))
				}
				if err := os.Link(dest, p); err != nil {
					return err
				}
			case typ == tar.TypeSymlink:
				dest := filepath.Join(filepath.Dir(p), hdr.Linkname)
				if !strings.HasPrefix(dest, dir) {
					return insecureLinkError(fmt.Errorf("insecure symlink %q -> %q", p, hdr.Linkname))
				}
				if err := os.Symlink(hdr.Linkname, p); err != nil {
					return err
				}
			case typ == tar.TypeChar:
				dev := makedev(int(hdr.Devmajor), int(hdr.Devminor))
				mode := uint32(fi.Mode()) | syscall.S_IFCHR
				if err := syscall.Mknod(p, mode, dev); err != nil {
					return err
				}
			case typ == tar.TypeBlock:
				dev := makedev(int(hdr.Devmajor), int(hdr.Devminor))
				mode := uint32(fi.Mode()) | syscall.S_IFBLK
				if err := syscall.Mknod(p, mode, dev); err != nil {
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

// makedev mimics glib's gnu_dev_makedev
func makedev(major, minor int) int {
	return (minor & 0xff) | (major & 0xfff << 8) | int((uint64(minor & ^0xff) << 12)) | int(uint64(major & ^0xfff)<<32)
}
