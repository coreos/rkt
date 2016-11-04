// Copyright 2014 Red Hat, Inc
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

package fileutil

// #include <sys/time.h>
import "C"

import (
	"fmt"
	"os"
	"syscall"
	"time"
)

func GetAtime(fi os.FileInfo) time.Time {
	stat := fi.Sys().(*syscall.Stat_t)
	return time.Unix(int64(stat.Atimespec.Sec), int64(stat.Atimespec.Nsec))
}

func GetCtime(fi os.FileInfo) time.Time {
	stat := fi.Sys().(*syscall.Stat_t)
	return time.Unix(int64(stat.Ctimespec.Sec), int64(stat.Ctimespec.Nsec))
}

func LUtimesNano(path string, ts []syscall.Timespec) error {
	timeval := C.struct_timeval{tv_sec: C.__darwin_time_t(ts[0].Sec), tv_usec: C.__darwin_suseconds_t(ts[0].Nsec / 1000)}

	if ret := C.lutimes(C.CString(path), &timeval); ret != 0 {
		return fmt.Errorf("Couldn't set timestamp on %s", path)
	}
	return nil
}

func hasHardLinks(fi os.FileInfo) bool {
	// On directories, Nlink doesn't make sense when checking for hard links
	return !fi.IsDir() && fi.Sys().(*syscall.Stat_t).Nlink > 1
}

func getInode(fi os.FileInfo) uint64 {
	return fi.Sys().(*syscall.Stat_t).Ino
}
