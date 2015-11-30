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

//+build linux

package stage0

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/schema/types"
)

// GC enters the pod by fork/exec()ing the stage1's /gc similar to /init.
// /gc can expect to have its CWD set to the pod root.
// stage1Path is the path of the stage1 rootfs
func GC(pdir string, uuid *types.UUID, stage1Path string, debug bool) error {
	err := unregisterPod(pdir, uuid)
	if err != nil {
		// Probably not worth abandoning the rest
		log.Printf("Warning: could not unregister pod with metadata service: %v", err)
	}

	ep, err := getStage1Entrypoint(pdir, gcEntrypoint)
	if err != nil {
		return fmt.Errorf("error determining 'gc' entrypoint: %v", err)
	}

	args := []string{filepath.Join(stage1Path, ep)}
	if debug {
		args = append(args, "--debug")
	}
	args = append(args, uuid.String())

	c := exec.Cmd{
		Path:   args[0],
		Args:   args,
		Stderr: os.Stderr,
		Dir:    pdir,
	}
	return c.Run()
}
