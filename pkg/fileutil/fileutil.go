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

package fileutil

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/pkg/device"
)

func copyRegularFile(src, dest string) (err error) {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()
	destFile, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer func() {
		e := destFile.Close()
		if err == nil {
			err = e
		}
	}()
	if _, err := io.Copy(destFile, srcFile); err != nil {
		return err
	}
	return nil
}

func copySymlink(src, dest string) error {
	symTarget, err := os.Readlink(src)
	if err != nil {
		return err
	}
	if err := os.Symlink(symTarget, dest); err != nil {
		return err
	}
	return nil
}

func CopyTree(src, dest string, uidShift uint64, uidCount uint64) error {
	dirs := make(map[string][]syscall.Timespec)
	copyWalker := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rootLess := path[len(src):]
		target := filepath.Join(dest, rootLess)
		mode := info.Mode()
		switch {
		case mode.IsDir():
			err := os.Mkdir(target, mode.Perm())
			if err != nil {
				return err
			}

			dir, err := os.Open(target)
			if err != nil {
				return err
			}
			if err := dir.Chmod(mode); err != nil {
				dir.Close()
				return err
			}
			dir.Close()
		case mode.IsRegular():
			if err := copyRegularFile(path, target); err != nil {
				return err
			}
		case mode&os.ModeSymlink == os.ModeSymlink:
			if err := copySymlink(path, target); err != nil {
				return err
			}
		case mode&os.ModeCharDevice == os.ModeCharDevice:
			stat := syscall.Stat_t{}
			if err := syscall.Stat(path, &stat); err != nil {
				return err
			}

			dev := device.Makedev(device.Major(stat.Rdev), device.Minor(stat.Rdev))
			mode := uint32(mode) | syscall.S_IFCHR
			if err := syscall.Mknod(target, mode, int(dev)); err != nil {
				return err
			}
		case mode&os.ModeDevice == os.ModeDevice:
			stat := syscall.Stat_t{}
			if err := syscall.Stat(path, &stat); err != nil {
				return err
			}

			dev := device.Makedev(device.Major(stat.Rdev), device.Minor(stat.Rdev))
			mode := uint32(mode) | syscall.S_IFBLK
			if err := syscall.Mknod(target, mode, int(dev)); err != nil {
				return err
			}
		case mode&os.ModeNamedPipe == os.ModeNamedPipe:
			if err := syscall.Mkfifo(target, uint32(mode)); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported mode: %v", mode)
		}

		var srcUid uint64 = uint64(info.Sys().(*syscall.Stat_t).Uid)
		var srcGid uint64 = uint64(info.Sys().(*syscall.Stat_t).Gid)

		if uidCount != 0 && (srcUid >= uidCount || srcGid >= uidCount) {
			return fmt.Errorf("source contains out of range uid/gid")
		}

		if err := os.Lchown(target, int(srcUid+uidShift), int(srcGid+uidShift)); err != nil {
			return err
		}

		// lchown(2) says that, depending on the linux kernel version, it
		// can change the file's mode also if executed as root. So call
		// os.Chmod after it.
		if mode&os.ModeSymlink != os.ModeSymlink {
			if err := os.Chmod(target, mode); err != nil {
				return err
			}
		}

		ts, err := pathToTimespec(path)
		if err != nil {
			return err
		}

		if mode.IsDir() {
			dirs[target] = ts
		}
		if mode&os.ModeSymlink != os.ModeSymlink {
			if err := syscall.UtimesNano(target, ts); err != nil {
				return err
			}
		} else {
			if err := LUtimesNano(target, ts); err != nil {
				return err
			}
		}

		return nil
	}

	if err := filepath.Walk(src, copyWalker); err != nil {
		return err
	}

	// Restore dirs atime and mtime. This has to be done after copying
	// as a file copying will change its parent directory's times.
	for dirPath, ts := range dirs {
		if err := syscall.UtimesNano(dirPath, ts); err != nil {
			return err
		}
	}

	return nil
}

func pathToTimespec(name string) ([]syscall.Timespec, error) {
	fi, err := os.Lstat(name)
	if err != nil {
		return nil, err
	}
	mtime := fi.ModTime()
	stat := fi.Sys().(*syscall.Stat_t)
	atime := time.Unix(int64(stat.Atim.Sec), int64(stat.Atim.Nsec))
	return []syscall.Timespec{TimeToTimespec(atime), TimeToTimespec(mtime)}, nil
}

// TODO(sgotti) use UTIMES_OMIT on linux if Time.IsZero ?
func TimeToTimespec(time time.Time) (ts syscall.Timespec) {
	nsec := int64(0)
	if !time.IsZero() {
		nsec = time.UnixNano()
	}
	return syscall.NsecToTimespec(nsec)
}
