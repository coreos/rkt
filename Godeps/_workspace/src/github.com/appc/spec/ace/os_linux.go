// +build linux

package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func checkMountImpl(d string, readonly bool) error {
	mountinfoPath := fmt.Sprintf("/proc/self/mountinfo")
	mi, err := os.Open(mountinfoPath)
	if err != nil {
		return err
	}
	defer mi.Close()

	sc := bufio.NewScanner(mi)
	for sc.Scan() {
		var (
			mountID        int
			parentID       int
			majorMinor     string
			root           string
			mountPoint     string
			mountOptions   string
			optionalFields string
			separator      string
			fsType         string
			mountSrc       string
			superOptions   string
		)

		_, err := fmt.Sscanf(sc.Text(), "%d %d %s %s %s %s %s %s %s %s %s",
			&mountID, &parentID, &majorMinor, &root, &mountPoint, &mountOptions,
			&optionalFields, &separator, &fsType, &mountSrc, &superOptions)
		if err != nil {
			return err
		}

		if mountPoint == d {
			var ro bool
			optionsParts := strings.Split(mountOptions, ",")
			for _, o := range optionsParts {
				switch o {
				case "ro":
					ro = true
				case "rw":
					ro = false
				}
			}
			if ro == readonly {
				return nil
			} else {
				return fmt.Errorf("%q mounted ro=%t, want %t", d, ro, readonly)
			}
		}
	}

	return fmt.Errorf("%q is not a mount point", d)
}
