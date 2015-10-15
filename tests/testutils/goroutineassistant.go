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

package testutils

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/steveeJ/gexpect"
)

type GoroutineAssistant struct {
	s  chan error
	wg sync.WaitGroup
	t  *testing.T
}

func NewGoroutineAssistant(t *testing.T) *GoroutineAssistant {
	return &GoroutineAssistant{
		s: make(chan error),
		t: t,
	}
}

func (a *GoroutineAssistant) Fatalf(s string, args ...interface{}) {
	a.wg.Done()
	a.s <- fmt.Errorf(s, args...)
}

func (a *GoroutineAssistant) Add(n int) {
	a.wg.Add(n)
}

func (a *GoroutineAssistant) Done() {
	a.wg.Done()
}

func (a *GoroutineAssistant) Wait() {
	go func() {
		a.wg.Wait()
		a.s <- nil
	}()
	err := <-a.s
	if err == nil {
		// success
		return
	}
	// If we received an error, let's fatal with that one. But for clean
	// test teardown, we need to allow the other goroutines to shut down.
	// We log any other errors we encounter in the meantime.
	a.t.Logf("Error encountered - shutting down")
	defer a.t.Fatal(err)
	teardown := time.After(5 * time.Minute)
	for {
		select {
		case <-teardown:
			a.t.Error("  timed out waiting for other goroutines to shut down!")
			return
		case err1 := <-a.s:
			if err1 == nil {
				// Clean shutdown!
				return
			}
			// Otherwise, log the other error
			a.t.Errorf("  additional error received while waiting for shutdown: %v", err1)
		}
	}
}

func (a *GoroutineAssistant) WaitOrFail(child *gexpect.ExpectSubprocess) {
	err := child.Wait()
	if err != nil {
		a.Fatalf("rkt didn't terminate correctly: %v", err)
	}
}

func (a *GoroutineAssistant) SpawnOrFail(cmd string) *gexpect.ExpectSubprocess {
	a.t.Logf("Command: %v", cmd)
	child, err := gexpect.Spawn(cmd)
	if err != nil {
		a.Fatalf("Cannot exec rkt: %v", err)
	}
	return child
}
