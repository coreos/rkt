// Copyright 2015 The rkt Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// +build darwin

package sys

// #include <sys/param.h>
// #include <sys/mount.h>
// #include <fcntl.h>
// #include <mach-o/dyld.h>
// int openAtWrapper(int fd, const char *path, int flags) {
//      return openat(fd, path, flags, 0);
// }
import "C"

import (
	"fmt"
	"syscall"
	"unsafe"
)

// Syncfs flushes uncommitted data by using the Fsync syscall
func Syncfs(fd int) error {
	return syscall.Fsync(fd)
}

func Mountfs(source string, target string, fstype string, flags uintptr, data string) error {
	ret := C.mount(C.CString(fstype), C.CString(target), C.int(flags), unsafe.Pointer(&source))
	if ret != 0 {
		fmt.Errorf("Couldn't mount %s to %s", source, target)
	}
	return nil
}

// not implemented
func RemountPrivate(target string) error {
	return ErrNotImplemented
}

func OpenAt(dfd int, path string, flags int) (int, error) {
	fd := C.openAtWrapper(C.int(dfd), C.CString(path), C.int(flags))
	if fd == -1 {
		return -1, fmt.Errorf("Couldn't open %s at %d", path, dfd)
	}
	return int(fd), nil
}

// SelfPath returns the path to executable runnning it
func SelfPath() (string, error) {
	path := make([]byte, 4096)
	ln := C.uint32_t(len(path))
	if ret := C._NSGetExecutablePath(C.CString(string(path)), &ln); ret != 0 {
		return "", fmt.Errorf("Couldn't determine executable path: %d", int(ret))
	}
	return string(path), nil
}
