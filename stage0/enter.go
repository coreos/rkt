//+build linux

package stage0

//
// Rocket is a reference implementation of the app container specification.
//
// Execution on Rocket is divided into a number of stages, and the `rkt`
// binary implements the first stage (stage 0)
//

import (
	"fmt"
	"os"
	"syscall"
)

const (
	enterPath = "stage1/enter"
)

// Enter enters the container by exec()ing the stage1's /enter similar to /init
// /enter can expect to have its CWD set to the container root
// imageID and command are supplied to /enter on argv followed by any arguments
func Enter(dir string, imageID string, cmdline []string) error {
	if err := os.Chdir(dir); err != nil {
		return fmt.Errorf("failed changing to dir: %v", err)
	}

	argv := []string{enterPath}
	argv = append(argv, imageID)
	argv = append(argv, cmdline...)
	if err := syscall.Exec(enterPath, argv, os.Environ()); err != nil {
		return fmt.Errorf("error execing enter: %v", err)
	}

	// never reached
	return nil
}
