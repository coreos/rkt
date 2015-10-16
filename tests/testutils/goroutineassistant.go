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
	"runtime"
	"sync"
	"testing"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/steveeJ/gexpect"
)

type GoroutineAssistant struct {
	wg sync.WaitGroup
	t  *testing.T

	m sync.Mutex
	s chan error
}

func NewGoroutineAssistant(t *testing.T) *GoroutineAssistant {
	return &GoroutineAssistant{
		s: make(chan error),
		t: t,
	}
}

func (a *GoroutineAssistant) notify(err error) {
	a.m.Lock()
	defer a.m.Unlock()
	if a.s != nil {
		a.s <- err
		close(a.s)
		a.s = nil
	}
}

// Fatalf should be used inside a goroutine instead of t.Fatalf. It
// quits the goroutine. Wait should handle the error.
func (a *GoroutineAssistant) Fatalf(s string, args ...interface{}) {
	a.notify(fmt.Errorf(s, args...))
	runtime.Goexit()
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
		a.notify(nil)
	}()
	if err := <-a.s; err != nil {
		a.t.Fatalf("%v", err)
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

func (a *GoroutineAssistant) WaitOrFail(child *gexpect.ExpectSubprocess, shouldSucceed bool) {
	err := child.Wait()
	switch {
	case !shouldSucceed && err == nil:
		a.Fatalf("Expected test to fail but it didn't")
	case shouldSucceed && err != nil:
		a.Fatalf("rkt didn't terminate correctly: %v", err)
	case err != nil && err.Error() != "exit status 1":
		a.Fatalf("rkt terminated with unexpected error: %v", err)
	}
}
