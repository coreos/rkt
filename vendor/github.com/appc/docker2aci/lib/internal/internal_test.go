// Copyright 2017 The appc Authors
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

package internal

import (
	"testing"

	"github.com/appc/spec/schema/types"
)

func TestSetLabel(t *testing.T) {
	labels := make(map[types.ACIdentifier]string)

	tests := []struct {
		key, value string
		ok         bool
	}{
		{"", "amd64", false},
		{"freebsd", "", false},
		{"", "", false},
		{"version", "1.2.3", true},
		{"os", "linux", true},
		{"arch", "aarch64", true},
		{"arch", "amd64", true},
	}

	for i, tt := range tests {
		setLabel(labels, tt.key, tt.value)

		value, ok := labels[types.ACIdentifier(tt.key)]
		if ok != tt.ok {
			const text = "#%d failed on label existence validation: %v != %v"
			t.Errorf(text, i, ok, tt.ok)
		}

		if tt.ok && value != tt.value {
			const text = "#%d wrong label for %s key: %v != %v"
			t.Errorf(text, i, tt.key, value, tt.value)
		}
	}
}

func TestSetAnnotation(t *testing.T) {
	var annotations types.Annotations

	tests := []struct {
		key, value string
		ok         bool
	}{
		{"", "", false},
		{"", "name", false},
		{"gentoo", "", false},
		{"entrypoint", "/bin/bash", true},
		{"entrypoint", "/bin/sh", true},
		{"cmd", "-c", true},
	}

	for i, tt := range tests {
		setAnnotation(&annotations, tt.key, tt.value)

		value, ok := annotations.Get(tt.key)
		if ok != tt.ok {
			const text = "#%d failed on annotation existence validation: %v != %v"
			t.Errorf(text, i, ok, tt.ok)
		}

		if tt.ok && value != tt.value {
			const text = "#%d wrong annotation for %s key: %v != %v"
			t.Errorf(text, i, tt.key, value, tt.value)
		}
	}
}

func TestOSArch(t *testing.T) {
	tests := []struct {
		srcOS, srcArch string
		dstOS, dstArch string
		err            bool
	}{
		{"", "", "", "", false},
		{"TempleOS", "ia64", "", "", false},
		{"linux", "amd64", "linux", "amd64", true},
		{"linux", "arm64", "linux", "aarch64", true},
		{"freebsd", "386", "freebsd", "i386", true},
	}

	for i, tt := range tests {
		labels := make(map[types.ACIdentifier]string)
		err := setOSArch(labels, tt.srcOS, tt.srcArch)

		if tt.err != (err == nil) {
			const text = "#%d unexpected result of os/arch conversion: %v"
			t.Errorf(text, i, err)
		}

		if labels["os"] != tt.dstOS {
			const text = "#%d expected %v os, got %v instead"
			t.Errorf(text, i, tt.dstOS, labels["os"])
		}

		if labels["arch"] != tt.dstArch {
			const text = "#%d expected %v arch, got %v instead"
			t.Errorf(text, i, tt.dstArch, labels["arch"])
		}
	}
}
