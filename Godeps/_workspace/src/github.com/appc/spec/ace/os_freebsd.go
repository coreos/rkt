// +build freebsd

package main

import "syscall"

func isSameFilesystem(a, b *syscall.Statfs_t) bool {
	if a.Fsid != (syscall.Fsid{}) || b.Fsid != (syscall.Fsid{}) {
		// If Fsid is not empty, we can just compare the IDs
		return a.Fsid == b.Fsid
	}
	// Fsids are zero, this happens in jails, but we can compare the rest
	return a.Fstypename == b.Fstypename &&
		a.Mntfromname == b.Mntfromname &&
		a.Mntonname == b.Mntonname
}
