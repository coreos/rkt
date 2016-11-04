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

// +build linux

package sys

import (
	"os"
	"syscall"
)

func Syncfs(fd int) error {
	_, _, err := syscall.RawSyscall(SYS_SYNCFS, uintptr(fd), 0, 0)
	if err != 0 {
		return syscall.Errno(err)
	}
	return nil
}

func Mountfs(source string, target string, fstype string, flags uintptr, data string) error {
	return syscall.Mount(source, target, fstype, flags, data)
}

func RemountPrivate(target string) error {
	return Mountfs("", target, "", syscall.MS_PRIVATE, "")
}

func OpenAt(dfd int, path string, flags int) (int, error) {
	return syscall.Openat(dfd, path, flags, 0)
}

// SelfPath returns the path to executable runnning it
func SelfPath() (string, error) {
	return os.Readlink("/proc/self/exe")
}
