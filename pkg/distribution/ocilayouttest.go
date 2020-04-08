// Copyright 2018 The rkt Authors
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
package distribution

import (
	"net/url"
	"testing"
)

func TestOCILayout(t *testing.T) {
	tests := []struct {
		ociRef       string
		expectedCIMD string
	}{
		{
			"https://demo.com/busybox.tar",
			"cimd:oci-layout:v=0:https://demo.com/busybox.tar",
		},
	}
	for _, tt := range tests {
		u, err := url.Parse(tt.ociRef)
		if err != nil {
			t.Errorf("unexpected err %v", err)
		}
		d, err := NewOCILayoutFromTransport(u)
		if err != nil {
			t.Errorf("unexpected err %v", err)
		}

		if d.CIMD().String() != tt.expectedCIMD {
			t.Errorf("CIMD got %s but want %s", d.CIMD().String(), tt.expectedCIMD)
		}
	}
}
