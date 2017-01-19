// Copyright 2016 The rkt Authors
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

//+build linux

// runc-prepare is a simple binary run as the first hook in a runc
// application. It takes care of some small cleanups. The rootfs should
// be passed as the first arg. It is executed in the stage1's rootfs.
package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Printf("Usage: %s <rootfs>\n", os.Args[0])
		os.Exit(1)
	}

	appRootfs := os.Args[1]
	os.Exit(prepare(appRootfs))
}

func prepare(appRootfs string) int {
	// /dev/log -> systemd
	for from, to := range map[string]string{
		"dev/log": "/run/systemd/journal/dev-log",
	} {
		from := filepath.Join(appRootfs, from)
		if err := os.Remove(from); err != nil && !os.IsNotExist(err) {
			fmt.Printf("rm %s failed: %s", from, err)
			return 2
		}
		if err := os.Symlink(to, from); err != nil {
			fmt.Printf("ln %s -> %s failed: %s", from, to, err)
			return 3
		}
	}

	return 0
}
