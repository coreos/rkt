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
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/coreos/rkt/networking/netinfo"
	"github.com/coreos/rkt/pkg/lock"
)

const (
	kvmSettingsDir        = "kvm"
	kvmPrivateKeyFilename = "ssh_kvm_key"
	// TODO: overwrite below default by environment value + generate .socket unit just before pod start
	kvmSSHPort = "122" // hardcoded value in .socket file
)

var (
	podPid  string
	appName string
)

func init() {
	flag.StringVar(&podPid, "pid", "", "podPID")
	flag.StringVar(&appName, "appname", "", "application to use")
}

func getPodDefaultIP(workDir string) (string, error) {
	// get pod lock
	l, err := lock.NewLock(workDir, lock.Dir)
	if err != nil {
		return "", err
	}

	// get file descriptor for lock
	fd, err := l.Fd()
	if err != nil {
		return "", err
	}

	// use this descriptor as method of reading pod network configuration
	nets, err := netinfo.LoadAt(fd)
	if err != nil {
		return "", err
	}
	// kvm flavored container must have at first position default vm<->host network
	if len(nets) == 0 {
		return "", fmt.Errorf("pod has no configured networks")
	}

	for _, net := range nets {
		if net.NetName == "default" || net.NetName == "default-restricted" {
			return net.IP.String(), nil
		}
	}

	return "", fmt.Errorf("pod has no default network!")
}

func getAppexecArgs() []string {
	// Documentation/devel/stage1-implementors-guide.md#arguments-1
	// also from ../enter/enter.c
	args := []string{
		"/appexec",
		fmt.Sprintf("/opt/stage2/%s/rootfs", appName),
		"/", // as in ../enter/enter.c - this should be app.WorkingDirectory
		fmt.Sprintf("/rkt/env/%s", appName),
		"0", // uid
		"0", // gid
	}
	return append(args, flag.Args()...)
}

func execSSH() error {
	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot get working directory: %v", err)
	}

	podDefaultIP, err := getPodDefaultIP(workDir)
	if err != nil {
		return fmt.Errorf("cannot load networking configuration: %v", err)
	}

	// escape from running pod directory into base directory
	if err = os.Chdir("../../.."); err != nil {
		return fmt.Errorf("cannot change directory to rkt work directory: %v", err)
	}

	// find path to ssh binary
	sshPath, err := exec.LookPath("ssh")
	if err != nil {
		return fmt.Errorf("cannot find 'ssh' binary in PATH: %v", err)
	}

	// prepare args for ssh invocation
	keyFile := filepath.Join(kvmSettingsDir, kvmPrivateKeyFilename)
	args := []string{
		"ssh",
		"-t",          // use tty
		"-i", keyFile, // use keyfile
		"-l", "root", // login as user
		"-p", kvmSSHPort, // port to connect
		"-o", "StrictHostKeyChecking=no", // do not check changing host keys
		"-o", "UserKnownHostsFile=/dev/null", // do not add host key to default knownhosts file
		"-o", "LogLevel=quiet", // do not log minor informations
		podDefaultIP,
	}
	args = append(args, getAppexecArgs()...)

	// this should not return in case of success
	err = syscall.Exec(sshPath, args, os.Environ())
	return fmt.Errorf("cannot exec to ssh: %v", err)
}

func main() {
	flag.Parse()
	if appName == "" {
		fmt.Fprintf(os.Stderr, "--appname not set to correct value\n")
		os.Exit(1)
	}

	// execSSH should returns only with error
	fmt.Fprintf(os.Stderr, "%v\n", execSSH())
	os.Exit(2)
}
