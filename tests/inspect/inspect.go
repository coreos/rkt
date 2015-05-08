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

package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"syscall"
	"time"
	"unsafe"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/syndtr/gocapability/capability"
)

var (
	globalFlagset = flag.NewFlagSet("inspect", flag.ExitOnError)
	globalFlags   = struct {
		ReadStdin    bool
		CheckTty     bool
		PrintMsg     string
		PrintEnv     string
		PrintCapsPid int
		PrintUser    bool
		CheckCwd     string
		ExitCode     int
		ReadFile     bool
		WriteFile    bool
		Sleep        int
	}{}
)

func init() {
	globalFlagset.BoolVar(&globalFlags.ReadStdin, "read-stdin", false, "Read a line from stdin")
	globalFlagset.BoolVar(&globalFlags.CheckTty, "check-tty", false, "Check if stdin is a terminal")
	globalFlagset.StringVar(&globalFlags.PrintMsg, "print-msg", "", "Print the message given as parameter")
	globalFlagset.StringVar(&globalFlags.CheckCwd, "check-cwd", "", "Check if the current working directory is the one specified")
	globalFlagset.StringVar(&globalFlags.PrintEnv, "print-env", "", "Print the specified environment variable")
	globalFlagset.IntVar(&globalFlags.PrintCapsPid, "print-caps-pid", -1, "Print capabilities of the specified pid (or current process if pid=0)")
	globalFlagset.BoolVar(&globalFlags.PrintUser, "print-user", false, "Print uid and gid")
	globalFlagset.IntVar(&globalFlags.ExitCode, "exit-code", 0, "Return this exit code")
	globalFlagset.BoolVar(&globalFlags.ReadFile, "read-file", false, "Print the content of the file $FILE")
	globalFlagset.BoolVar(&globalFlags.WriteFile, "write-file", false, "Write $CONTENT in the file $FILE")
	globalFlagset.IntVar(&globalFlags.Sleep, "sleep", -1, "Sleep before exiting (in seconds)")
}

func main() {
	globalFlagset.Parse(os.Args[1:])
	args := globalFlagset.Args()
	if len(args) > 0 {
		fmt.Fprintln(os.Stderr, "Wrong parameters")
		os.Exit(1)
	}

	if globalFlags.ReadStdin {
		reader := bufio.NewReader(os.Stdin)
		fmt.Printf("Enter text:\n")
		text, _ := reader.ReadString('\n')
		fmt.Printf("Received text: %s\n", text)
	}

	if globalFlags.CheckTty {
		fd := int(os.Stdin.Fd())
		var termios syscall.Termios
		_, _, err := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(fd), syscall.TCGETS, uintptr(unsafe.Pointer(&termios)), 0, 0, 0)
		if err == 0 {
			fmt.Printf("stdin is a terminal\n")
		} else {
			fmt.Printf("stdin is not a terminal\n")
		}
	}

	if globalFlags.PrintMsg != "" {
		fmt.Fprintf(os.Stdout, "%s\n", globalFlags.PrintMsg)
	}

	if globalFlags.PrintEnv != "" {
		fmt.Fprintf(os.Stdout, "%s=%s\n", globalFlags.PrintEnv, os.Getenv(globalFlags.PrintEnv))
	}

	if globalFlags.PrintCapsPid >= 0 {
		caps, err := capability.NewPid(globalFlags.PrintCapsPid)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Cannot get caps: %v\n", err)
			os.Exit(1)
			return
		}
		fmt.Printf("Capability set: effective: %s\n", caps.StringCap(capability.EFFECTIVE))
		fmt.Printf("Capability set: permitted: %s\n", caps.StringCap(capability.PERMITTED))
		fmt.Printf("Capability set: inheritable: %s\n", caps.StringCap(capability.INHERITABLE))
		fmt.Printf("Capability set: bounding: %s\n", caps.StringCap(capability.BOUNDING))

		if capStr := os.Getenv("CAPABILITY"); capStr != "" {
			capInt, err := strconv.Atoi(capStr)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Environment variable $CAPABILITY is not a valid capability number: %v\n", err)
				os.Exit(1)
				return
			}
			c := capability.Cap(capInt)
			if caps.Get(capability.BOUNDING, c) {
				fmt.Printf("%v=enabled\n", c.String())
			} else {
				fmt.Printf("%v=disabled\n", c.String())
			}
		}
	}

	if globalFlags.PrintUser {
		fmt.Printf("User: uid=%d euid=%d gid=%d egid=%d\n", os.Getuid(), os.Geteuid(), os.Getgid(), os.Getegid())
	}

	if globalFlags.WriteFile {
		fileName := os.Getenv("FILE")
		err := ioutil.WriteFile(fileName, []byte(os.Getenv("CONTENT")), 0600)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Cannot write to file %q: %v\n", fileName, err)
			os.Exit(1)
			return
		}
	}

	if globalFlags.ReadFile {
		fileName := os.Getenv("FILE")
		dat, err := ioutil.ReadFile(fileName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Cannot read file %q: %v\n", fileName, err)
			os.Exit(1)
			return
		}
		fmt.Print("<<<")
		fmt.Print(string(dat))
		fmt.Print(">>>\n")
	}

	if globalFlags.CheckCwd != "" {
		wd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Cannot get working directory: %v\n", err)
			os.Exit(1)
		}
		if wd != globalFlags.CheckCwd {
			fmt.Fprintf(os.Stderr, "Working directory: %q. Expected: %q.\n", wd, globalFlags.CheckCwd)
			os.Exit(1)
		}
	}

	if globalFlags.Sleep >= 0 {
		time.Sleep(time.Duration(globalFlags.Sleep) * time.Second)
	}

	os.Exit(globalFlags.ExitCode)
}
