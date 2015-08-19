// +build !linux
// +build !freebsd

package main

import (
	"syscall"
)

func isSameFilesystem(a, b *syscall.Statfs_t) bool {
	return a.Fsid == b.Fsid
}
