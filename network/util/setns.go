package util

import (
	"fmt"
	"os"
	"runtime"
	"syscall"
)

var setNsMap = map[string]uintptr{
	"386":   346,
	"amd64": 308,
	"arm":   374,
}

func SetNS(f *os.File, flags uintptr) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	trap, ok := setNsMap[runtime.GOARCH]
	if !ok {
		return fmt.Errorf("unsupported arch: %s", runtime.GOARCH)
	}
	_, _, err := syscall.RawSyscall(trap, f.Fd(), flags, 0)
	if err != 0 {
		return err
	}

	return nil
}
