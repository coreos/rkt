// Copyright 2014 CoreOS, Inc.
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
	"strings"
	"testing"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/schema"
	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/schema/types"
)

func TestQuoteExec(t *testing.T) {
	tests := []struct {
		input  []string
		output string
	}{
		{
			input:  []string{`path`, `"arg1"`, `"'arg2'"`, `'arg3'`},
			output: `path "\"arg1\"" "\"\'arg2\'\"" "\'arg3\'"`,
		}, {
			input:  []string{`path`},
			output: `path`,
		}, {
			input:  []string{`path`, ``, `arg2`},
			output: `path "" "arg2"`,
		}, {
			input:  []string{`path`, `"foo\bar"`, `\`},
			output: `path "\"foo\\bar\"" "\\"`,
		},
	}

	for i, tt := range tests {
		o := quoteExec(tt.input)
		if o != tt.output {
			t.Errorf("#%d: expected `%v` got `%v`", i, tt.output, o)
		}
	}
}

// TestAppToNspawnArgsOverridesImageManifestReadOnly tests
// that the ImageManifest's `readOnly` volume setting will be
// overrided by PodManifest.
func TestAppToNspawnArgsOverridesImageManifestReadOnly(t *testing.T) {
	falseVar := false
	trueVar := true
	tests := []struct {
		imageManifestVolumeReadOnly bool
		podManifestVolumeReadOnly   *bool
		expectReadOnly              bool
	}{
		{
			false,
			nil,
			false,
		},
		{
			false,
			&falseVar,
			false,
		},
		{
			false,
			&trueVar,
			true,
		},
		{
			true,
			nil,
			true,
		},
		{
			true,
			&falseVar,
			false,
		},
		{
			true,
			&trueVar,
			true,
		},
	}

	for i, tt := range tests {
		imageManifest := &schema.ImageManifest{
			App: &types.App{
				MountPoints: []types.MountPoint{
					{
						Name:     "foo-mount",
						Path:     "/app/foo",
						ReadOnly: tt.imageManifestVolumeReadOnly,
					},
				},
			},
		}
		podManifest := &schema.PodManifest{
			Volumes: []types.Volume{
				{
					Name:     "foo-mount",
					Kind:     "host",
					Source:   "/host/foo",
					ReadOnly: tt.podManifestVolumeReadOnly,
				},
			},
		}
		appManifest := &schema.RuntimeApp{
			Mounts: []schema.Mount{
				{
					Volume:     "foo-mount",
					MountPoint: "foo-mount",
				},
			},
		}

		p := &Pod{Manifest: podManifest}
		output, err := p.appToNspawnArgs(appManifest, imageManifest)
		if err != nil {
			t.Errorf("#%d: unexpected error: `%v`", i, err)
		}
		if ro := strings.HasPrefix(output[0], "--bind-ro"); ro != tt.expectReadOnly {
			t.Errorf("#%d: expected: readOnly: %v, saw: %v", i, tt.expectReadOnly, ro)
		}
	}
}
