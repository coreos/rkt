// Copyright 2017 The rkt Authors
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
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"os/signal"
    "strings"
	"syscall"

	"github.com/appc/spec/schema/types"

	"github.com/coreos/rkt/common"
	pkgflag "github.com/coreos/rkt/pkg/flag"
	rktlog "github.com/coreos/rkt/pkg/log"
	"github.com/coreos/rkt/pkg/sys"
	"github.com/coreos/rkt/pkg/user"
	stage1common "github.com/coreos/rkt/stage1/common"
	stage1commontypes "github.com/coreos/rkt/stage1/common/types"
)

const (
	flavor = "skim"
)

var (
	debug bool

	discardNetlist common.NetList
	discardBool    bool
	discardString  string

	pids	[]int

	log  *rktlog.Logger
	diag *rktlog.Logger
)


func parseFlags() *stage1commontypes.RuntimePod {
	rp := stage1commontypes.RuntimePod{}

	flag.BoolVar(&debug, "debug", false, "Run in debug mode")

	// The following flags need to be supported by stage1 according to
	// https://github.com/coreos/rkt/blob/master/Documentation/devel/stage1-implementors-guide.md
	// Most of them are ignored
	// These are ignored, but stage0 always passes them
	flag.Var(&discardNetlist, "net", "Setup networking")
	flag.StringVar(&discardString, "local-config", common.DefaultLocalConfigDir, "Local config path")

	// These are discarded with a warning
	// TODO either implement these, or stop passing them
	flag.Bool("interactive", true, "The pod is interactive (ignored, always true)")
	flag.Var(pkgflag.NewDiscardFlag("mds-token"), "mds-token", "MDS auth token (not implemented)")

	flag.Var(pkgflag.NewDiscardFlag("hostname"), "hostname", "Set hostname (not implemented)")
	flag.Bool("disable-capabilities-restriction", true, "ignored")
	flag.Bool("disable-paths", true, "ignored")
	flag.Bool("disable-seccomp", true, "ignored")

    // Since we're running on the host natively, we wll also ingnore tweaking dns/host
	dnsConfMode := pkgflag.MustNewPairList(map[string][]string{
		"resolv": {"host", "stage0", "none", "default"},
		"hosts":  {"host", "stage0", "default"},
	}, map[string]string{
		"resolv": "default",
		"hosts":  "default",
	})
	flag.Var(dnsConfMode, "dns-conf-mode", "DNS config file modes")

	flag.Parse()

	rp.Debug = debug

	return &rp
}

/**
 * Reap all processes that belong to this pod. I don't believe this should be
 * necessary once we migrate towards systemd
 */
func reapChildren() {
	/* remove the SIGCHLD handler so we don't trip over it inadvertently */
	signal.Reset(syscall.SIGCHLD)

	for _, p := range pids {
		proc, err := os.FindProcess(p)
		if err != nil {
			fmt.Printf("Cannot find: %d skipping\n", p)
			continue
		}

		err = proc.Kill(); if err != nil {
			fmt.Printf("Error killing: %d skipping\n", p)
		}
	}
}

func savePids() error {
	result := ""
	for _, v := range pids {
		result += fmt.Sprintf("%d\n", v)
	}

	return ioutil.WriteFile("childpids", bytes.NewBufferString(result).Bytes(), 644)
}

func stage1(rp *stage1commontypes.RuntimePod) int {
	rootDir,_ := os.Getwd()

	uuid, err := types.NewUUID(flag.Arg(0))
	if err != nil {
		log.Print("UUID is missing or malformed\n")
		return 254
	}

	root := "."
	p, err := stage1commontypes.LoadPod(root, uuid, rp)
	if err != nil {
		log.PrintE("can't load pod", err)
		return 254
	}

	if err := p.SaveRuntime(); err != nil {
		log.FatalE("failed to save runtime parameters", err)
	}

	// CAB: Generate the rkt-UUID-system.slice
	// From there, the following daemons will want this particular slice

	// lock the current goroutine to its current OS thread.
	// This will force the subsequent syscalls to be executed in the same OS thread as Setresuid, and Setresgid,
	// see https://github.com/golang/go/issues/1435#issuecomment-66054163.
	runtime.LockOSThread()

	lfd, err := common.GetRktLockFD()
	if err != nil {
		log.PrintE("can't get rkt lock fd", err)
		return 254
	}

	defer reapChildren()
	childChannel := make(chan os.Signal, 1)
	signal.Notify(childChannel, syscall.SIGCHLD, os.Interrupt)

	for _, ra := range p.Manifest.Apps {
		imgName := p.AppNameToImageName(ra.Name)
		args := ra.App.Exec
		if len(args) == 0 {
			log.Printf(`image %q has an empty "exec" (try --exec=BINARY)`, imgName)
			return 254
		}

	    // change permissions for the root directory to be world readable/executable
	    // This is to ensure external ancillary scripts work without having to be
	    // root or setuid-root
	    err = os.Chmod(common.AppPath(p.Root, ra.Name), 0755); if err != nil {
	        log.Error(err)
	        return 254
	    }

		workDir := "/"
		if ra.App.WorkingDirectory != "" {
			workDir = ra.App.WorkingDirectory
		}

		rfs := filepath.Join(common.AppPath(p.Root, ra.Name), "rootfs")
		pid := os.Getpid()

		if err = stage1common.WritePid(pid, "pid"); err != nil {
			log.Error(err)
			return 254
		}

		var uidResolver, gidResolver user.Resolver
		var uid, gid int

		uidResolver, err = user.NumericIDs(ra.App.User)
		if err != nil {
			uidResolver, err = user.IDsFromStat(rfs, ra.App.User, nil)
		}

		if err != nil { // give up
			log.PrintE(fmt.Sprintf("invalid user %q", ra.App.User), err)
			return 254
		}

		if uid, _, err = uidResolver.IDs(); err != nil {
			log.PrintE(fmt.Sprintf("failed to configure user %q", ra.App.User), err)
			return 254
		}

		gidResolver, err = user.NumericIDs(ra.App.Group)
		if err != nil {
			gidResolver, err = user.IDsFromStat(rfs, ra.App.Group, nil)
		}

		if err != nil { // give up
			log.PrintE(fmt.Sprintf("invalid group %q", ra.App.Group), err)
			return 254
		}

		if _, gid, err = gidResolver.IDs(); err != nil {
			log.PrintE(fmt.Sprintf("failed to configure group %q", ra.App.Group), err)
			return 254
		}

		diag.Printf("setting uid %d gid %d", uid, gid)

		if err := syscall.Setresgid(gid, gid, gid); err != nil {
			log.PrintE(fmt.Sprintf("can't set gid %d", gid), err)
			return 254
		}

		if err := syscall.Setresuid(uid, uid, uid); err != nil {
			log.PrintE(fmt.Sprintf("can't set uid %d", uid), err)
			return 254
		}

	    // Update the runtime path to reflect the absolute path of the container
		path := "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
		execDir := filepath.Join(rootDir, rfs)

	    var containerPath string
	    for _, p := range strings.Split(path, ":") {
	        containerPath += execDir + workDir + p + ":"
	    }

		env := []string{"PATH=" + containerPath + path}
		for _, e := range ra.App.Environment {
			env = append(env, e.Name+"="+e.Value)
		}

		diag.Printf("spawning %q in %q", args, execDir)
		var procAttr syscall.ProcAttr
		procAttr.Env = env
		procAttr.Files = [](uintptr){os.Stdin.Fd(), os.Stdout.Fd(), os.Stderr.Fd()}
		procAttr.Dir = execDir
		pid, err = syscall.ForkExec(filepath.Join(execDir,args[0]), args, &procAttr)
		if err != nil {
			log.PrintE(fmt.Sprintf("can't execute %q/%q", execDir, args[0]), err)
			return 254
		}

		pids = append(pids, pid)
	}

	/* Reset our working directory */
	err = savePids(); if err != nil {
		log.PrintE("can't save pids", err)
	}

	// clear close-on-exec flag on RKT_LOCK_FD, to keep pod status as running after exec().
	if err := sys.CloseOnExec(lfd, false); err != nil {
		log.PrintE("unable to clear FD_CLOEXEC on pod lock", err)
		return 254
	}

	// exec into our stage1-sync program to allow everything else to bind to it
	syncCmd := filepath.Join(common.Stage1RootfsPath(rootDir), "sync")
	diag.Printf("execing stage1-sync: %q\n", syncCmd)
	if err = syscall.Exec(syncCmd, nil, nil); err != nil {
		log.PrintE("can't execute stage1-sync:", err)
		return 254
	}

	return 0
}

func main() {
	rp := parseFlags()

	log, diag, _ = rktlog.NewLogSet("run", debug)
	if !debug {
		diag.SetOutput(ioutil.Discard)
	}

	// move code into stage1() helper so defered fns get run
	os.Exit(stage1(rp))
}
