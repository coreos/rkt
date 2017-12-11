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

// Disabled on kvm due to https://github.com/rkt/rkt/issues/3382
// +build !fly,!kvm

package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/coreos/gexpect"
	"github.com/rkt/rkt/tests/testutils"
)

const (
	// Number of retries to read log file.
	logsReadRetries = 5
	// Delay between each retry attempt in reading log file.
	logsReadRetryDelay = 5 * time.Second
)

// TestAppSandboxOneApp is a basic test for `rkt app` sandbox.
// It starts the sandbox, adds one app, starts it, and removes it.
func TestAppSandboxAddStartRemove(t *testing.T) {
	testSandbox(t, func(ctx *testutils.RktRunCtx, child *gexpect.ExpectSubprocess, podUUID string) {
		actionTimeout := 30 * time.Second
		imageName := "coreos.com/rkt-inspect/hello"
		appName := "hello-app"
		msg := "HelloFromAppInSandbox"

		aciHello := patchTestACI("rkt-inspect-hello.aci", "--name="+imageName, "--exec=/inspect --print-msg="+msg)
		defer os.Remove(aciHello)

		combinedOutput(t, ctx.ExecCmd("fetch", "--insecure-options=image", aciHello))
		combinedOutput(t, ctx.ExecCmd("app", "add", "--debug", podUUID, imageName, "--name="+appName))
		combinedOutput(t, ctx.ExecCmd("app", "start", "--debug", podUUID, "--app="+appName))

		if err := expectTimeoutWithOutput(child, msg, actionTimeout); err != nil {
			t.Fatalf("Expected %q but not found: %v", msg, err)
		}

		combinedOutput(t, ctx.ExecCmd("app", "rm", "--debug", podUUID, "--app="+appName))

		out := combinedOutput(t, ctx.ExecCmd("app", "list", "--no-legend", podUUID))
		if out != "\n" {
			t.Errorf("unexpected output %q", out)
			return
		}
	})
}

// TestAppSandboxAddDefaultAppName tests applying default names to apps.
// It starts the sandbox, adds one app and ensures that it has a name converted from
// the image name.
func TestAppSandboxAddDefaultAppName(t *testing.T) {
	testSandbox(t, func(ctx *testutils.RktRunCtx, child *gexpect.ExpectSubprocess, podUUID string) {
		imageName := "coreos.com/rkt-inspect/hello"
		msg := "HelloFromAppInSandbox"

		expectedAppName := "hello"

		aciHello := patchTestACI("rkt-inspect-hello.aci", "--name="+imageName, "--exec=/inspect --print-msg="+msg)
		defer os.Remove(aciHello)

		combinedOutput(t, ctx.ExecCmd("fetch", "--insecure-options=image", aciHello))
		combinedOutput(t, ctx.ExecCmd("app", "add", "--debug", podUUID, imageName))

		podInfo := getPodInfo(t, ctx, podUUID)
		appName := podInfo.manifest.Apps[0].Name

		if appName.String() != expectedAppName {
			t.Errorf("got %s app name, expected %s app name", appName, expectedAppName)
		}
	})
}

// TestAppSandboxMultipleApps tests multiple apps in a sandbox:
// one that exits successfully, one that exits with an error, and one that keeps running.
func TestAppSandboxMultipleApps(t *testing.T) {
	testSandbox(t, func(ctx *testutils.RktRunCtx, child *gexpect.ExpectSubprocess, podUUID string) {
		actionTimeout := 30 * time.Second

		type app struct {
			name, image, exec, aci string
		}

		apps := []app{
			{
				name:  "winner",
				image: "coreos.com/rkt-inspect/success",
				exec:  "/inspect -print-msg=SUCCESS",
				aci:   "rkt-inspect-success.aci",
			},
			{
				name:  "loser",
				image: "coreos.com/rkt-inspect/fail",
				exec:  "/inspect -print-msg=FAILED -exit-code=12",
				aci:   "rkt-inspect-loser.aci",
			},
			{
				name:  "sleeper",
				image: "coreos.com/rkt-inspect/sleep",
				exec:  "/inspect -print-msg=SLEEP -sleep=120",
				aci:   "rkt-inspect-sleep.aci",
			},
		}

		// create, fetch, add, and start all apps in the sandbox
		for _, app := range apps {
			aci := patchTestACI(app.aci, "--name="+app.image, "--exec="+app.exec)
			defer os.Remove(aci)

			combinedOutput(t, ctx.ExecCmd("fetch", "--insecure-options=image", aci))
			combinedOutput(t, ctx.ExecCmd("app", "add", "--debug", podUUID, app.image, "--name="+app.name))
			combinedOutput(t, ctx.ExecCmd("app", "start", "--debug", podUUID, "--app="+app.name))
		}

		// check for app output messages
		for _, msg := range []string{
			"SUCCESS",
			"FAILED",
			"SLEEP",
		} {
			if err := expectTimeoutWithOutput(child, msg, actionTimeout); err != nil {
				t.Fatalf("Expected %q but not found: %v", msg, err)
			}
		}

		// total retry timeout: 10s
		r := retry{
			n: 20,
			t: 500 * time.Millisecond,
		}

		// assert `rkt app list` for the apps
		if err := r.Retry(func() error {
			got := combinedOutput(t, ctx.ExecCmd("app", "list", "--no-legend", podUUID))

			if strings.Contains(got, "winner\texited") &&
				strings.Contains(got, "loser\texited") &&
				strings.Contains(got, "sleeper\trunning") {
				return nil
			}

			return fmt.Errorf("unexpected result, got %q", got)
		}); err != nil {
			t.Error(err)
			return
		}

		// assert `rkt app status` for the apps
		for _, app := range []struct {
			name          string
			checkExitCode bool
			exitCode      int
			state         string
		}{
			{
				name:          "winner",
				checkExitCode: true,
				exitCode:      0,
				state:         "exited",
			},
			{
				name:          "loser",
				checkExitCode: true,
				exitCode:      12,
				state:         "exited",
			},
			{
				name:  "sleeper",
				state: "running",
			},
		} {
			if err := r.Retry(func() error {
				got := combinedOutput(t, ctx.ExecCmd("app", "status", podUUID, "--app="+app.name))
				ok := true

				if app.checkExitCode {
					ok = ok && strings.Contains(got, "exit_code="+strconv.Itoa(app.exitCode))
				}

				ok = ok && strings.Contains(got, "state="+app.state)

				if !ok {
					return fmt.Errorf("unexpected result, got %q", got)
				}

				return nil
			}); err != nil {
				t.Error(err)
				return
			}
		}

		// remove all apps
		for _, app := range apps {
			combinedOutput(t, ctx.ExecCmd("app", "rm", "--debug", podUUID, "--app="+app.name))
		}

		// assert empty `rkt app list`, no need for retrying,
		// as after removal no leftovers are expected to be present
		got := combinedOutput(t, ctx.ExecCmd("app", "list", "--no-legend", podUUID))
		if got != "\n" {
			t.Errorf("unexpected result, got %q", got)
			return
		}
	})
}

// TestAppSandboxRestart tests multiple apps in a sandbox and restarts one of them.
func TestAppSandboxRestart(t *testing.T) {
	testSandbox(t, func(ctx *testutils.RktRunCtx, child *gexpect.ExpectSubprocess, podUUID string) {
		type app struct {
			name, image, exec, aci string
		}

		apps := []app{
			{
				name:  "app1",
				image: "coreos.com/rkt-inspect/app1",
				exec:  "/inspect -sleep=120",
				aci:   "rkt-inspect-app1.aci",
			},
			{
				name:  "app2",
				image: "coreos.com/rkt-inspect/app1",
				exec:  "/inspect -sleep=120",
				aci:   "rkt-inspect-app1.aci",
			},
		}

		// create, fetch, add, and start all apps in the sandbox
		for _, app := range apps {
			aci := patchTestACI(app.aci, "--name="+app.image, "--exec="+app.exec)
			defer os.Remove(aci)

			combinedOutput(t, ctx.ExecCmd("fetch", "--insecure-options=image", aci))
			combinedOutput(t, ctx.ExecCmd("app", "add", "--debug", podUUID, app.image, "--name="+app.name))
			combinedOutput(t, ctx.ExecCmd("app", "start", "--debug", podUUID, "--app="+app.name))
		}

		// total retry timeout: 10s
		r := retry{
			n: 20,
			t: 500 * time.Millisecond,
		}

		// assert `rkt app list` for the apps
		if err := r.Retry(func() error {
			got := combinedOutput(t, ctx.ExecCmd("app", "list", "--no-legend", podUUID))

			if strings.Contains(got, "app1\trunning") &&
				strings.Contains(got, "app2\trunning") {
				return nil
			}

			return fmt.Errorf("unexpected result, got %q", got)
		}); err != nil {
			t.Error(err)
			return
		}

		assertStatus := func(name, status string) error {
			return r.Retry(func() error {
				got := combinedOutput(t, ctx.ExecCmd("app", "status", podUUID, "--app="+name))

				if !strings.Contains(got, status) {
					return fmt.Errorf("unexpected result, got %q", got)
				}

				return nil
			})
		}

		// assert `rkt app status` for the apps
		for _, app := range apps {
			if err := assertStatus(app.name, "state=running"); err != nil {
				t.Error(err)
				return
			}
		}

		// stop app1
		combinedOutput(t, ctx.ExecCmd("app", "stop", podUUID, "--app=app1"))

		// assert `rkt app status` for the apps
		for _, app := range []struct {
			name   string
			status string
		}{
			{
				name:   "app1",
				status: "state=exited",
			},
			{
				name:   "app2",
				status: "state=running",
			},
		} {
			if err := assertStatus(app.name, app.status); err != nil {
				t.Error(err)
				return
			}
		}

		// assert `rkt app list` for the apps
		if err := r.Retry(func() error {
			got := combinedOutput(t, ctx.ExecCmd("app", "list", "--no-legend", podUUID))

			if strings.Contains(got, "app1\texited") &&
				strings.Contains(got, "app2\trunning") {
				return nil
			}

			return fmt.Errorf("unexpected result, got %q", got)
		}); err != nil {
			t.Error(err)
			return
		}

		// start app1
		combinedOutput(t, ctx.ExecCmd("app", "start", podUUID, "--app=app1"))

		// assert `rkt app status` for the apps
		for _, app := range []struct {
			name   string
			status string
		}{
			{
				name:   "app1",
				status: "state=running",
			},
			{
				name:   "app2",
				status: "state=running",
			},
		} {
			if err := assertStatus(app.name, app.status); err != nil {
				t.Error(err)
				return
			}
		}

		// assert `rkt app list` for the apps
		if err := r.Retry(func() error {
			got := combinedOutput(t, ctx.ExecCmd("app", "list", "--no-legend", podUUID))

			if strings.Contains(got, "app1\trunning") &&
				strings.Contains(got, "app2\trunning") {
				return nil
			}

			return fmt.Errorf("unexpected result, got %q", got)
		}); err != nil {
			t.Error(err)
			return
		}
	})
}

func TestAppSandboxMount(t *testing.T) {
	// this test hast to be skipped on semaphore for now,
	// because it uses an outdated kernel hindering mount propagation,
	// letting this test fail.
	if os.Getenv("SEMAPHORE") == "true" {
		t.Skip("skipped on semaphore")
	}

	mntSrcDir := mustTempDir("rkt-mount-test-")
	defer os.RemoveAll(mntSrcDir)

	mntSrcFile := filepath.Join(mntSrcDir, "test")
	if err := ioutil.WriteFile(mntSrcFile, []byte("content"), 0666); err != nil {
		t.Fatalf("Cannot write file: %v", err)
	}

	testSandbox(t, func(ctx *testutils.RktRunCtx, child *gexpect.ExpectSubprocess, podUUID string) {
		aci := patchTestACI(
			"rkt-inspect-mounter.aci",
			"--name=coreos.com/rkt-inspect/mounter",
			"--exec=/inspect -read-file",
		)
		defer os.Remove(aci)
		combinedOutput(t, ctx.ExecCmd("fetch", "--insecure-options=image", aci))

		for _, tt := range []struct {
			mntTarget    string
			expectedFile string
		}{
			{
				mntTarget:    "/dir2",
				expectedFile: "/dir2/test",
			},
			{
				mntTarget:    "/dir1/link_rel_dir2",
				expectedFile: "/dir2/test",
			},
			{
				mntTarget:    "/dir1/link_abs_dir2",
				expectedFile: "/dir2/test",
			},
			{
				mntTarget:    "/dir1/link_abs_root/notexists",
				expectedFile: "/notexists/test",
			},
			{
				mntTarget:    "/../../../../../../../../notexists",
				expectedFile: "/notexists/test",
			},
			{
				mntTarget:    "../../../../../../../../notexists",
				expectedFile: "/notexists/test",
			},
		} {
			combinedOutput(t, ctx.ExecCmd(
				"app", "add", "--debug", podUUID,
				"coreos.com/rkt-inspect/mounter",
				"--name=mounter",
				"--environment=FILE="+tt.expectedFile,
				"--mnt-volume=name=test,kind=host,source="+mntSrcDir+",target="+tt.mntTarget,
			))

			combinedOutput(t, ctx.ExecCmd("app", "start", "--debug", podUUID, "--app=mounter"))

			if err := expectTimeoutWithOutput(child, "content", 10*time.Second); err != nil {
				t.Fatalf("Expected \"content\" but not found: %v", err)
			}

			combinedOutput(t, ctx.ExecCmd("app", "rm", "--debug", podUUID, "--app=mounter"))
		}
	})
}

func TestAppSandboxAnnotations(t *testing.T) {
	for _, tt := range []struct {
		args                    []string
		expectedAnnotations     map[string]string
		expectedUserAnnotations map[string]string
	}{
		{
			args: []string{
				"--annotation=foo=bar",
				"--user-annotation=ayy=lmao",
			},
			expectedAnnotations:     map[string]string{"foo": "bar"},
			expectedUserAnnotations: map[string]string{"ayy": "lmao"},
		},
	} {
		testSandboxWithArgs(t, tt.args, func(ctx *testutils.RktRunCtx, child *gexpect.ExpectSubprocess, podUUID string) {
			podInfo := getPodInfo(t, ctx, podUUID)

			annotations := make(map[string]string)
			for _, annotation := range podInfo.manifest.Annotations {
				annotations[annotation.Name.String()] = annotation.Value
			}

			userAnnotations := make(map[string]string)
			for k, v := range podInfo.manifest.UserAnnotations {
				userAnnotations[k] = v
			}

			// Annotations contain both the data from CLI and data generated dufing rkt runtime.
			// We check here only for the data we provided with CLi.
			for k, v := range tt.expectedAnnotations {
				value, ok := annotations[k]
				if !ok {
					t.Errorf("key %s doesn't exist in annotations", k)
				}
				if v != value {
					t.Errorf("got value %s, expected value %s", v, value)
				}
			}

			if !reflect.DeepEqual(userAnnotations, tt.expectedUserAnnotations) {
				t.Errorf("got %v user annotations, expected %v user annotations", userAnnotations, tt.expectedUserAnnotations)
			}
		})
	}
}

func TestAppSandboxAppAnnotations(t *testing.T) {
	testSandbox(t, func(ctx *testutils.RktRunCtx, child *gexpect.ExpectSubprocess, podUUID string) {
		for _, tt := range []struct {
			args                    []string
			expectedAnnotations     map[string]string
			expectedUserAnnotations map[string]string
		}{
			{
				args: []string{
					"--annotation=foo=bar",
					"--user-annotation=ayy=lmao",
				},
				expectedAnnotations:     map[string]string{"foo": "bar"},
				expectedUserAnnotations: map[string]string{"ayy": "lmao"},
			},
		} {
			imageName := "coreos.com/rkt-inspect/hello"
			msg := "HelloFromAppInSandbox"

			aciHello := patchTestACI("rkt-inspect-hello.aci", "--name="+imageName, "--exec=/inspect --print-msg="+msg)
			defer os.Remove(aciHello)

			combinedOutput(t, ctx.ExecCmd("fetch", "--insecure-options=image", aciHello))

			args := []string{
				"app", "add", "--debug", podUUID,
				"coreos.com/rkt-inspect/hello",
				"--name=annotation-test",
			}
			args = append(args, tt.args...)

			combinedOutput(t, ctx.ExecCmd(args...))

			podInfo := getPodInfo(t, ctx, podUUID)

			annotations := make(map[string]string)
			for _, annotation := range podInfo.manifest.Apps[0].Annotations {
				annotations[annotation.Name.String()] = annotation.Value
			}

			userAnnotations := make(map[string]string)
			for k, v := range podInfo.manifest.Apps[0].App.UserAnnotations {
				userAnnotations[k] = v
			}

			if !reflect.DeepEqual(annotations, tt.expectedAnnotations) {
				t.Errorf("expected %v annotations, got %v annotations", annotations, tt.expectedAnnotations)
			}
			if !reflect.DeepEqual(userAnnotations, tt.expectedUserAnnotations) {
				t.Errorf("expected %v user annotations, got %v user annotations", userAnnotations, tt.expectedUserAnnotations)
			}
		}
	})
}

func TestAppSandboxCRILogs(t *testing.T) {
	if TestedFlavor.Kvm || TestedFlavor.Fly {
		t.Skip("CRI logs are not supported in kvm and fly flavors yet")
	}

	for _, tt := range []struct {
		kubernetesLogDir  string
		kubernetesLogPath string
	}{
		{
			kubernetesLogDir:  "/tmp/rkt-test-logs",
			kubernetesLogPath: "hello_0.log",
		},
	} {
		args := []string{
			"--annotation=coreos.com/rkt/experiment/logmode=k8s-plain",
			fmt.Sprintf("--annotation=coreos.com/rkt/experiment/kubernetes-log-dir=%s", tt.kubernetesLogDir),
		}

		if err := os.MkdirAll(tt.kubernetesLogDir, 0777); err != nil {
			t.Fatalf("Couldn't create directory for CRI logs: %v", err)
		}
		defer os.RemoveAll(tt.kubernetesLogDir)

		testSandboxWithArgs(t, args, func(ctx *testutils.RktRunCtx, child *gexpect.ExpectSubprocess, podUUID string) {
			imageName := "coreos.com/rkt-inspect/hello"
			msg := "HelloFromAppInSandbox"

			aciHello := patchTestACI("rkt-inspect-hello.aci", "--name="+imageName, "--exec=/inspect --print-msg="+msg)
			defer os.Remove(aciHello)

			combinedOutput(t, ctx.ExecCmd("fetch", "--insecure-options=image", aciHello))

			combinedOutput(t, ctx.ExecCmd(
				"app", "add", "--debug", podUUID,
				imageName, "--name=hello", "--stdin=stream",
				"--stdout=stream", "--stderr=stream",
				fmt.Sprintf("--annotation=coreos.com/rkt/experiment/kubernetes-log-path=%s", tt.kubernetesLogPath),
			))
			combinedOutput(t, ctx.ExecCmd("app", "start", "--debug", podUUID, "--app=hello"))

			// It takes some time to have iottymux unit running inside the stage1 container,
			// so we are looking for CRI logs with a reasonable amount of retries.
			var content []byte
			var err error
			for i := 0; i < logsReadRetries; i++ {
				kubernetesLogFullPath := path.Join(tt.kubernetesLogDir, tt.kubernetesLogPath)
				content, err = ioutil.ReadFile(kubernetesLogFullPath)
				if err == nil {
					sContent := string(content)
					if strings.Contains(sContent, "stdout HelloFromAppInSandbox") {
						break
					}
					err = fmt.Errorf("Expected CRI logs to contain 'stdout HelloFromAppInSandbox', instead got: %s", sContent)
				} else {
					err = fmt.Errorf("Couldn't open file with CRI logs: %v", err)
				}
				time.Sleep(logsReadRetryDelay)
			}
			if err != nil {
				t.Fatal(err)
			}

		})
	}
}

func TestAppSandboxJSONFileLogs(t *testing.T) {
	if TestedFlavor.Kvm || TestedFlavor.Fly {
		t.Skip("json-file logs are not supported in kvm and fly flavors yet")
	}

	for _, tt := range []struct {
		jsonLogDir  string
		jsonLogFile string
	}{
		{
			jsonLogDir:  "/tmp/rkt-test-logs",
			jsonLogFile: "hello_0.log",
		},
	} {
		args := []string{
			"--annotation=coreos.com/rkt/experiment/logmode=json-file",
			fmt.Sprintf("--annotation=coreos.com/rkt/experiment/json-log-dir=%s", tt.jsonLogDir),
		}

		if err := os.MkdirAll(tt.jsonLogDir, 0777); err != nil {
			t.Fatalf("Couldn't create directory for JSON logs: %v", err)
		}
		defer os.RemoveAll(tt.jsonLogDir)

		testSandboxWithArgs(t, args, func(ctx *testutils.RktRunCtx, child *gexpect.ExpectSubprocess, podUUID string) {
			imageName := "coreos.com/rkt-inspect/hello"
			msg := "HelloFromAppInSandbox"

			aciHello := patchTestACI("rkt-inspect-hello.aci", "--name="+imageName, "--exec=/inspect --print-msg="+msg)
			defer os.Remove(aciHello)

			combinedOutput(t, ctx.ExecCmd("fetch", "--insecure-options=image", aciHello))

			combinedOutput(t, ctx.ExecCmd(
				"app", "add", "--debug", podUUID,
				imageName, "--name=hello", "--stdin=stream",
				"--stdout=stream", "--stderr=stream",
				fmt.Sprintf("--annotation=coreos.com/rkt/experiment/json-log-file=%s", tt.jsonLogFile),
			))
			combinedOutput(t, ctx.ExecCmd("app", "start", "--debug", podUUID, "--app=hello"))

			// It takes some time to have iottymux unit running inside the stage1 container,
			// so we are looking for json logs with a reasonable amount of retries.
			// same as CRI logs just above
			var content []byte
			var err error
            var sContent string
			for i := 0; i < logsReadRetries; i++ {
                err = nil
				jsonLogFullPath := path.Join(tt.jsonLogDir, tt.jsonLogFile)
				content, err = ioutil.ReadFile(jsonLogFullPath)
                if err == nil {
                    sContent = string(content)
                    if len(sContent) > 0 {
                        break
                    }
                }
				time.Sleep(logsReadRetryDelay)
			}
            if err != nil {
                t.Fatal(err)
            }
            if !(strings.Contains(sContent, "\"log\":\"HelloFromAppInSandbox\\n\"") && strings.Contains(sContent, "\"stream\":\"stdout\"")) {
                t.Fatalf("Expected json logs to contain '\"log\":\"HelloFromAppInSandbox\\n\"' and '\"stream\":\"stdout\"', instead got: %s", sContent)
            }

		})
	}
}

func testSandbox(t *testing.T, testFunc func(*testutils.RktRunCtx, *gexpect.ExpectSubprocess, string)) {
	testSandboxWithArgs(t, nil, testFunc)
}

func testSandboxWithArgs(t *testing.T, args []string, testFunc func(*testutils.RktRunCtx, *gexpect.ExpectSubprocess, string)) {
	if err := os.Setenv("RKT_EXPERIMENT_APP", "true"); err != nil {
		panic(err)
	}
	defer os.Unsetenv("RKT_EXPERIMENT_APP")

	if err := os.Setenv("RKT_EXPERIMENT_ATTACH", "true"); err != nil {
		panic(err)
	}
	defer os.Unsetenv("RKT_EXPERIMENT_ATTACH")

	tmpDir := mustTempDir("rkt-test-cri-")
	uuidFile := filepath.Join(tmpDir, "uuid")
	defer os.RemoveAll(tmpDir)

	ctx := testutils.NewRktRunCtx()
	defer ctx.Cleanup()

	rkt := ctx.Cmd() + " app sandbox --uuid-file-save=" + uuidFile
	if args != nil {
		rkt = rkt + " " + strings.Join(args, " ")
	}
	child := spawnOrFail(t, rkt)

	// wait for the sandbox to start
	podUUID, err := waitPodReady(ctx, t, uuidFile, 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}

	testFunc(ctx, child, podUUID)

	// assert that the pod is still running
	got := combinedOutput(t, ctx.ExecCmd("status", podUUID))
	if !strings.Contains(got, "state=running") {
		t.Errorf("unexpected result, got %q", got)
		return
	}

	combinedOutput(t, ctx.ExecCmd("stop", podUUID))

	waitOrFail(t, child, 0)
}
