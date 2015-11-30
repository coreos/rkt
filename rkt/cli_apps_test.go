// Copyright 2014 The rkt Authors
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
	"reflect"
	"strings"
	"testing"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/schema/types"
	flag "github.com/coreos/rkt/Godeps/_workspace/src/github.com/spf13/pflag"
)

func TestParseAppArgs(t *testing.T) {
	flags := flag.NewFlagSet("test", flag.ExitOnError)
	flags.SetInterspersed(false)
	tests := []struct {
		in     string
		images []string
		args   [][]string
		werr   bool
	}{
		{
			"example.com/foo example.com/bar -- --help --- example.com/baz -- --verbose",
			[]string{"example.com/foo", "example.com/bar", "example.com/baz"},
			[][]string{
				nil,
				[]string{"--help"},
				[]string{"--verbose"},
			},
			false,
		},
		{
			"example.com/foo --- example.com/bar --- example.com/baz ---",
			[]string{"example.com/foo", "example.com/bar", "example.com/baz"},
			[][]string{
				nil,
				nil,
				nil,
			},
			false,
		},
		{
			"example.com/foo example.com/bar example.com/baz",
			[]string{"example.com/foo", "example.com/bar", "example.com/baz"},
			[][]string{
				nil,
				nil,
				nil,
			},
			false,
		},
	}

	for i, tt := range tests {
		rktApps.Reset()
		err := parseApps(&rktApps, strings.Split(tt.in, " "), flags, true)
		ga := rktApps.GetArgs()
		gi := rktApps.GetImages()
		if gerr := (err != nil); gerr != tt.werr {
			t.Errorf("#%d: err==%v, want errstate %t", i, err, tt.werr)
		}
		if !reflect.DeepEqual(ga, tt.args) {
			t.Errorf("#%d: got args %v, want args %v", i, ga, tt.args)
		}
		if !reflect.DeepEqual(gi, tt.images) {
			t.Errorf("#%d: got images %v, want images %v", i, gi, tt.images)
		}
	}

}

func TestParsePortFlag(t *testing.T) {
	tests := []struct {
		in  string
		ex  types.ExposedPort
		err bool
	}{
		{
			in: "foo:123",
			ex: types.ExposedPort{
				Name:     "foo",
				HostPort: 123,
			},
			err: false,
		},
		{
			in:  "f$o:123",
			ex:  types.ExposedPort{},
			err: true,
		},
		{
			in:  "foo:12345",
			ex:  types.ExposedPort{},
			err: true,
		},
	}

	for _, tt := range tests {
		pl := portList{}
		err := pl.Set(tt.in)

		if err != nil {
			if !tt.err {
				t.Errorf("%q failed to parse: %v", tt.in, err)
			}
			return
		}

		if tt.err {
			t.Errorf("%q unexpectedly parsed", tt.in)
			return
		}

		if len(pl) == 0 {
			t.Errorf("%q parsed into a empty list", tt.in)
			return
		}

		if pl[0].Name != tt.ex.Name {
			t.Errorf("%q parsed but Name mismatch: got %v, expected %v", tt.in, pl[0].Name, tt.ex.Name)
		}

		if pl[0].HostPort != tt.ex.HostPort {
			t.Errorf("%q parsed but HostPort mismatch: got %v, expected %v", tt.in, pl[0].HostPort, tt.ex.HostPort)
		}
	}
}

var options = []string{"zero", "one", "two"}

func TestOptionList(t *testing.T) {
	tests := []struct {
		opts string
		ex   string
		err  bool
	}{
		{
			opts: "zero,two",
			ex:   "zero,two",
			err:  false,
		},
		{ // Duplicate test
			opts: "one,two,two",
			ex:   "",
			err:  true,
		},
		{ // Not permissible test
			opts: "one,two,three",
			ex:   "",
			err:  true,
		},
		{ // Empty string
			opts: "",
			ex:   "",
			err:  false,
		},
	}

	for i, tt := range tests {
		// test newOptionsList
		if _, err := newOptionList(options, tt.opts); (err != nil) != tt.err {
			t.Errorf("test %d: unexpected error in newOptionList: %v", i, err)
		}

		// test optionList.Set()
		ol, err := newOptionList(options, strings.Join(options, ","))
		if err != nil {
			t.Errorf("test %d: unexpected error preparing test: %v", i, err)
		}

		if err := ol.Set(tt.opts); (err != nil) != tt.err {
			t.Errorf("test %d: could not parse options as expected: %v", i, err)
		}
		if tt.ex != "" && tt.ex != ol.String() {
			t.Errorf("test %d: resulting options not as expected: %s != %s",
				i, tt.ex, ol.String())
		}
	}
}

var bfMap = map[string]int{
	options[0]: 0,
	options[1]: 1,
	options[2]: 1 << 1,
}

func TestBitFlags(t *testing.T) {
	tests := []struct {
		opts     string
		ex       int
		parseErr bool
		logicErr bool
	}{
		{
			opts: "one,two",
			ex:   3,
		},
		{ // Duplicate test
			opts:     "zero,two,two",
			ex:       -1,
			parseErr: true,
		},
		{ // Not included test
			opts:     "zero,two,three",
			ex:       -1,
			parseErr: true,
		},
		{ // Test 10 in 11
			opts: "one,two",
			ex:   1,
		},
		{ // Test 11 not in 01
			opts:     "one",
			ex:       3,
			logicErr: true,
		},
	}

	for i, tt := range tests {
		// test NewBitFlags
		if _, err := newBitFlags(options, tt.opts, bfMap); (err != nil) != tt.parseErr {
			t.Errorf("test %d: unexpected error in newBitFlag: %v", i, err)
		}

		bf, err := newBitFlags(options, strings.Join(options, ","), bfMap)
		if err != nil {
			t.Errorf("test %d: unexpected error preparing test: %v", i, err)
		}

		// test BitFlags.Set()
		if err := bf.Set(tt.opts); (err != nil) != tt.parseErr {
			t.Errorf("test %d: Could not parse options as expected: %v", i, err)
		}
		if tt.ex >= 0 && bf.hasFlag(tt.ex) == tt.logicErr {
			t.Errorf("test %d: Result was unexpected: %d != %d",
				i, tt.ex, bf.flags)
		}
	}
}
