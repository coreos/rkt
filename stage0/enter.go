// Copyright 2014 CoreOS, Inc.
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
	"os"
	"path/filepath"
	"syscall"

	"github.com/coreos/rocket/Godeps/_workspace/src/github.com/appc/spec/schema/types"
	"github.com/coreos/rocket/common"
)

// Enter enters the container by exec()ing the stage1's /enter similar to /init
// /enter can expect to have its CWD set to the container root
// app and command are supplied to /enter on argv followed by any arguments
func Enter(cdir string, app *types.ACName, cmdline []string) error {
	if err := os.Chdir(cdir); err != nil {
		return fmt.Errorf("error changing to dir: %v", err)
	}

	ep, err := getStage1Entrypoint(cdir, enterEntrypoint)
	if err != nil {
		return fmt.Errorf("error determining entrypoint: %v", err)
	}

	argv := []string{filepath.Join(common.Stage1RootfsPath(cdir), ep)}
	argv = append(argv, app.String())
	argv = append(argv, cmdline...)
	if err := syscall.Exec(argv[0], argv, os.Environ()); err != nil {
		return fmt.Errorf("error execing enter: %v", err)
	}

	// never reached
	return nil
}
