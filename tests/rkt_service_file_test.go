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
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/coreos/go-systemd/dbus"
	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/coreos/go-systemd/util"
)

func TestServiceFile(t *testing.T) {
	if !util.IsRunningSystemd() {
		t.Skip("Systemd is not running on the host.")
	}

	ctx := newRktRunCtx()
	defer ctx.cleanup()

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	conn, err := dbus.New()
	if err != nil {
		t.Fatal(err)
	}

	imageFile := os.Getenv("RKT_INSPECT_IMAGE")
	if imageFile == "" {
		t.Fatal("RKT_INSPECT_IMAGE is not set")
	}
	image, err := filepath.Abs(imageFile)
	if err != nil {
		t.Fatal(err)
	}
	opts := "-- --print-msg=HelloWorld --sleep=1000"

	cmd := fmt.Sprintf("%s --insecure-skip-verify run --set-env=MESSAGE_LOOP=1000 %s %s", ctx.cmd(), image, opts)
	props := []dbus.Property{
		dbus.PropExecStart(strings.Split(cmd, " "), false),
	}
	target := fmt.Sprintf("rkt-testing-transient-%d.service", r.Int())

	reschan := make(chan string)
	_, err = conn.StartTransientUnit(target, "replace", props, reschan)
	if err != nil {
		t.Fatal(err)
	}

	job := <-reschan
	if job != "done" {
		t.Fatal("Job is not done:", job)
	}

	units, err := conn.ListUnits()

	var found bool
	for _, u := range units {
		if u.Name == target {
			found = true
			if u.ActiveState != "active" {
				t.Fatalf("Test unit %s not active: %s (target: %s)", u.Name, u.ActiveState, target)
			}
		}
	}

	if !found {
		t.Fatalf("Test unit not found in list")
	}

	// Run the unit for 10 seconds. You can check the logs manually in journalctl
	time.Sleep(10 * time.Second)

	// Stop the unit
	_, err = conn.StopUnit(target, "replace", reschan)
	if err != nil {
		t.Fatal(err)
	}

	// wait for StopUnit job to complete
	<-reschan

	units, err = conn.ListUnits()

	found = false
	for _, u := range units {
		if u.Name == target {
			found = true
		}
	}

	if found {
		t.Fatalf("Test unit found in list, should be stopped")
	}
}
