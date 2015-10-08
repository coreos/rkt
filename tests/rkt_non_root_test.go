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
	"fmt"
	"os"
	"runtime"
	"syscall"
	"testing"

	"github.com/coreos/rkt/common"
)

// TestNonRootReadInfo tests that non-root users that can do rkt list, rkt image list.
func TestNonRootReadInfo(t *testing.T) {
	runtime.LockOSThread()
	ctx := newRktRunCtx()
	defer ctx.cleanup()

	gid, err := common.LookupGid(common.RktGroup)
	if err != nil {
		t.Skipf("Skipping the test because there's no %q group", common.RktGroup)
	}

	// Do 'rkt install. and launch some pods in root'.
	cmd := fmt.Sprintf("%s install", ctx.cmd())
	t.Logf("Running rkt install")
	runRktAndCheckOutput(t, cmd, "rkt directory structure successfully created", false)

	// Launch some pods, this creates the environment for later testing,
	// also it exercises 'rkt install'.
	imgs := []struct {
		name     string
		msg      string
		exitCode string
		imgFile  string
	}{
		{name: "inspect-1", msg: "foo-1", exitCode: "1"},
		{name: "inspect-2", msg: "foo-2", exitCode: "2"},
		//{name: "inspect-3", msg: "foo-3", exitCode: "3"}, // Waiting for #1453 #1503.
	}

	for i, img := range imgs {
		imgName := fmt.Sprintf("rkt-%s.aci", img.name)
		imgs[i].imgFile = patchTestACI(imgName, fmt.Sprintf("--name=%s", img.name), fmt.Sprintf("--exec=/inspect --print-msg=%s --exit-code=%s", img.msg, img.exitCode))
		defer os.Remove(imgs[i].imgFile)
	}

	runCmds := []string{
		// Run with overlay, no private-users.
		fmt.Sprintf("%s --insecure-skip-verify run --mds-register=false %s", ctx.cmd(), imgs[0].imgFile),

		// Run without overlay, no private-users.
		fmt.Sprintf("%s --insecure-skip-verify run --no-overlay --mds-register=false %s", ctx.cmd(), imgs[1].imgFile),

		// Run without overlay and private-users.
		//fmt.Sprintf("%s --insecure-skip-verify run --no-overlay --private-users --mds-register=false %s", ctx.cmd(), imgs[2].imgFile),
	}

	for i, cmd := range runCmds {
		t.Logf("#%d: Running %s", i, cmd)
		runRktAndCheckOutput(t, cmd, imgs[i].msg, false)
	}

	done := make(chan struct{})

	// Go disables setuid/setgid because of its runtime model makes
	// these syscall not effactive in most cases because:
	// 1. setuid/setgid syscalls only affects the calling (kernel) thread.
	// 2. Kernel thread is not binded to goroutine.
	//
	// Here we use LockOSThread() and Syscall() to work around this.
	// Also we need to run in a groutine and destroy the kernel thread on exit
	// so we are able to clean up the context in root.
	// For more details, see: https://code.google.com/p/go/issues/detail?id=1435
	go func() {
		runtime.LockOSThread()
		defer func() {
			close(done)
			// Destroy the thread so that other goroutines will not run in non-root.
			syscall.Syscall(syscall.SYS_EXIT, uintptr(0), 0, 0)
		}()

		// Force runtime to create kernel thread for other goroutines if necessary.
		// This reduces the chance that the runtime clones this thread after we setuid/setgid.
		// (Hopefully, if no other goroutines are created during the execution of this goroutine,
		// then no more kernel threads will be created).
		runtime.Gosched()

		// Setgid to 'rkt'.
		_, _, e := syscall.Syscall(syscall.SYS_SETGID, uintptr(gid), 0, 0)
		if e != 0 {
			t.Fatalf("Cannot setgid: %v", e.Error())
		}

		// Setuid to 'nobody' (65534).
		_, _, e = syscall.Syscall(syscall.SYS_SETUID, uintptr(65534), 0, 0)
		if e != 0 {
			t.Fatalf("Cannot setuid: %v", e.Error())
		}

		// Run rkt list/status to check status.
		for _, img := range imgs {
			checkAppStatus(t, ctx, false, img.name, fmt.Sprintf("status=%s", img.exitCode))
		}

		// Run rkt image list to check images.
		imgListCmd := fmt.Sprintf("%s image list", ctx.cmd())
		t.Logf("Running %s", imgListCmd)
		runRktAndCheckOutput(t, imgListCmd, "inspect-", false)
	}()

	<-done
}

// TestNonRootFetchRmGcImage tests that non-root users can remove images fetched by themselves but
// can not remove images fetched by root, or gc any images.
func TestNonRootFetchRmGcImage(t *testing.T) {
	runtime.LockOSThread()

	ctx := newRktRunCtx()
	defer ctx.cleanup()

	gid, err := common.LookupGid(common.RktGroup)
	if err != nil {
		t.Skipf("Skipping the test because there's no %q group", common.RktGroup)
	}

	// Do 'rkt install and fetch an image with root.
	cmd := fmt.Sprintf("%s install", ctx.cmd())
	t.Logf("Running rkt install")
	runRktAndCheckOutput(t, cmd, "rkt directory structure successfully created", false)

	rootImg := patchTestACI("rkt-inspect-root-rm.aci", "--exec=/inspect --print-msg=foobar")
	defer os.Remove(rootImg)
	rootImgHash := importImageAndFetchHash(t, ctx, rootImg)

	// Launch/gc a pod so we can test non-root image gc.
	runCmd := fmt.Sprintf("%s --insecure-skip-verify run --mds-register=false %s", ctx.cmd(), rootImg)
	runRktAndCheckOutput(t, runCmd, "foobar", false)

	ctx.runGC()

	done := make(chan struct{})

	go func() {
		runtime.LockOSThread()
		defer func() {
			close(done)
			syscall.Syscall(syscall.SYS_EXIT, uintptr(0), 0, 0)
		}()

		runtime.Gosched()

		// Setgid to 'rkt'.
		_, _, e := syscall.Syscall(syscall.SYS_SETGID, uintptr(gid), 0, 0)
		if e != 0 {
			t.Fatalf("Cannot setgid: %v", e.Error())
		}

		// Setuid to 'nobody' (65534).
		_, _, e = syscall.Syscall(syscall.SYS_SETUID, uintptr(65534), 0, 0)
		if e != 0 {
			t.Fatalf("Cannot setuid: %v", e.Error())
		}

		// Should not be able to do image gc.
		imgGcCmd := fmt.Sprintf("%s image gc", ctx.cmd())
		t.Logf("Running %s", imgGcCmd)
		runRktAndCheckOutput(t, imgGcCmd, "permission denied", true)

		// Should not be able to remove the image fetched by root.
		imgRmCmd := fmt.Sprintf("%s image rm %s", ctx.cmd(), rootImgHash)
		t.Logf("Running %s", imgRmCmd)
		runRktAndCheckOutput(t, imgRmCmd, "permission denied", true)

		// Should be able to remove the image fetched by ourselves.
		nonrootImg := patchTestACI("rkt-inspect-non-root-rm.aci", "--exec=/inspect")
		defer os.Remove(nonrootImg)
		nonrootImgHash := importImageAndFetchHash(t, ctx, nonrootImg)

		imgRmCmd = fmt.Sprintf("%s image rm %s", ctx.cmd(), nonrootImgHash)
		t.Logf("Running %s", imgRmCmd)
		runRktAndCheckOutput(t, imgRmCmd, "successfully removed", false)
	}()

	<-done
}
