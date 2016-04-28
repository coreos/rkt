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

// +build skip

package main

import "testing"

func TestNetDefaultPortFwdConnectivity(t *testing.T) {
	NewNetDefaultPortFwdConnectivityTest(
		PortFwdCase{"172.16.28.1", "--net=default", true},
	).Execute(t)
}

func TestNetCustomPtp(t *testing.T) {
	// PTP means connection Point-To-Point. That is, connections to other pods/containers should be forbidden
	NewNetCustomPtpTest(false)
}
