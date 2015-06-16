// +build linux freebsd netbsd openbsd

package tarheader

import (
	"archive/tar"
	"os"
	"syscall"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/pkg/device"
)

func init() {
	populateHeaderStat = append(populateHeaderStat, populateHeaderUnix)
}

func populateHeaderUnix(h *tar.Header, fi os.FileInfo, seen map[uint64]string) {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return
	}
	h.Uid = int(st.Uid)
	h.Gid = int(st.Gid)
	if st.Mode&syscall.S_IFMT == syscall.S_IFBLK || st.Mode&syscall.S_IFMT == syscall.S_IFCHR {
		h.Devminor = int64(device.Minor(uint64(st.Rdev)))
		h.Devmajor = int64(device.Major(uint64(st.Rdev)))
	}
	// If we have already seen this inode, generate a hardlink
	p, ok := seen[uint64(st.Ino)]
	if ok {
		h.Linkname = p
		h.Typeflag = tar.TypeLink
	} else {
		seen[uint64(st.Ino)] = h.Name
	}
}
