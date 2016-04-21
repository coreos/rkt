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

package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"testing"

	cnitypes "github.com/appc/cni/pkg/types"
	"github.com/coreos/rkt/networking/netinfo"
	nettypes "github.com/coreos/rkt/networking/types"
)

type netConfigFile struct {
	filename string
	contents string
}

func TestOldConfigFormat(t *testing.T) {
	tmp := getTmpDir(t, "rkt-networking-old-conf-format")
	defer os.RemoveAll(tmp)
	tests := []struct {
		contents string
		expected Networks
		fail     bool
	}{
		{"bogus contents", Networks{}, true},
		// the old cni config is basically anything goes - if there is something that fits, cool
		{`{"bogus": {"foo": "bar"}}`, getSingleNetwork(getActiveNetwork("10-foo.conf", "", "")), false},
		{`{"name": ["should be a string, not an array"]`, Networks{}, true},
		{`{"name": "foo", "type": "bar"}`, getSingleNetwork(getActiveNetwork("10-foo.conf", "foo", "bar")), false},
	}
	for i, tt := range tests {
		ncf := &netConfigFile{
			filename: "10-foo.conf",
			contents: tt.contents,
		}
		cfgDir := generateOldNetworkConfig(t, tmp, i, ncf)
		if !tt.fail && tt.expected.Ordered != nil {
			fixupActiveNetwork(tt.expected.Ordered[0], cfgDir, tt.contents)
		}
		fixupConfPaths(tt.expected.Ordered)
		cfg, err := GetConfigFrom(cfgDir)
		if tt.fail {
			if err == nil {
				t.Errorf("expected the test %d to fail, it did not", i)
			}
			continue
		} else if err != nil {
			t.Errorf("test %d unexpectedly failed: %v", i, err)
			continue
		}
		if !reflect.DeepEqual(tt.expected, cfg.Networks) {
			// TODO: This really is not a useful output,
			// %#v does not follow pointers...
			t.Errorf("test %d failed, expected:\n%#v\ngot:\n%#v", i, tt.expected, cfg.Networks)
			continue
		}
	}
}

func TestOldConfigOverride(t *testing.T) {
	tmp := getTmpDir(t, "rkt-networking-old-conf-override")
	defer os.RemoveAll(tmp)
	tests := []struct {
		lsof     [][]*netConfigFile
		expected Networks
	}{
		{
			lsof: [][]*netConfigFile{
				[]*netConfigFile{
					&netConfigFile{
						filename: "10-foo.conf",
						contents: getJSONConfig("foo", "foo"),
					},
					&netConfigFile{
						filename: "20-bar.conf",
						contents: getJSONConfig("bar", "bar"),
					},
				},
			},
			expected: getNetworks(getValidActiveNetwork("10-foo.conf", "foo", "foo"), getValidActiveNetwork("20-bar.conf", "bar", "bar")),
		},
		{
			lsof: [][]*netConfigFile{
				[]*netConfigFile{
					&netConfigFile{
						filename: "10-foo.conf",
						contents: getJSONConfig("foo", "foo"),
					},
					&netConfigFile{
						filename: "20-foo.conf",
						contents: getJSONConfig("foo", "bar"),
					},
				},
			},
			expected: getNetworks(getValidActiveNetwork("10-foo.conf", "foo", "foo")),
		},
		{
			lsof: [][]*netConfigFile{
				[]*netConfigFile{
					&netConfigFile{
						filename: "10-global-foo.conf",
						contents: getJSONConfig("foo", "foo"),
					},
					&netConfigFile{
						filename: "20-global-bar.conf",
						contents: getJSONConfig("bar", "bar"),
					},
					&netConfigFile{
						filename: "40-global-baz.conf",
						contents: getJSONConfig("baz", "baz"),
					},
				},
				[]*netConfigFile{
					&netConfigFile{
						filename: "90-local-foo.conf",
						contents: getJSONConfig("foo", "oof"),
					},
					&netConfigFile{
						filename: "01-local-bar.conf",
						contents: getJSONConfig("bar", "rab"),
					},
					&netConfigFile{
						filename: "60-local-moo.conf",
						contents: getJSONConfig("moo", "moo"),
					},
				},
			},
			expected: getNetworks(getValidActiveNetwork("90-local-foo.conf", "foo", "oof"), getValidActiveNetwork("40-global-baz.conf", "baz", "baz"), getValidActiveNetwork("60-local-moo.conf", "moo", "moo"), getValidActiveNetwork("01-local-bar.conf", "bar", "rab")),
		},
	}
	for i, tt := range tests {
		var dirs []string
		for dirIdx, files := range tt.lsof {
			num := i*10 + dirIdx
			cfgDir := generateOldNetworkConfig(t, tmp, num, files...)
			for _, an := range tt.expected.Ordered {
				for _, file := range files {
					if an.Runtime.ConfPath == file.filename {
						fixupActiveNetwork(an, cfgDir, "")
						break
					}
				}
			}
			dirs = append(dirs, cfgDir)
		}
		fixupConfPaths(tt.expected.Ordered)
		cfg, err := GetConfigFrom(dirs...)
		if err != nil {
			t.Errorf("test %d unexpectedly failed: %v", i, err)
			continue
		}
		if !reflect.DeepEqual(tt.expected, cfg.Networks) {
			// TODO: This really is not a useful output,
			// %#v does not follow pointers...
			t.Errorf("test %d failed, expected:\n%#v\ngot:\n%#v", i, tt.expected, cfg.Networks)
			continue
		}
	}
}

func TestOldConfigUnique(t *testing.T) {
	tmp := getTmpDir(t, "rkt-networking-old-conf-unique")
	defer os.RemoveAll(tmp)
	ncfs := []*netConfigFile{
		&netConfigFile{
			filename: "10-foo.conf",
			contents: getJSONConfig("foo", "foo"),
		},
		&netConfigFile{
			filename: "10-foo.conf",
			contents: getJSONConfig("bar", "bar"),
		},
	}
	var dirs []string
	for i, ncf := range ncfs {
		cfgDir := generateOldNetworkConfig(t, tmp, i, ncf)
		dirs = append(dirs, cfgDir)
	}
	cfg, err := GetConfigFrom(dirs...)
	if err != nil {
		t.Fatalf("getting config unexpectedly failed: %v", err)
	}
	if len(cfg.Networks.Ordered) != 2 {
		t.Fatalf("expected to have 2 different network configs, got %d", len(cfg.Networks.Ordered))
	}
	an1 := cfg.Networks.Ordered[0]
	an2 := cfg.Networks.Ordered[1]
	b1 := filepath.Base(an1.Runtime.ConfPath)
	b2 := filepath.Base(an2.Runtime.ConfPath)
	if b1 == b2 {
		t.Fatalf("got two different network configs with the same filename, one of them will be clobbered by another when loading networks into a pod")
	}
}

func fixupActiveNetwork(an *nettypes.ActiveNet, cfgDir, contents string) {
	if an.ConfBytes == nil {
		an.ConfBytes = []byte(contents)
	}
	an.Runtime.ConfPath = filepath.Join(getOldNetDir(cfgDir), an.Runtime.ConfPath)
}

func getActiveNetwork(filename, netName, netType string) *nettypes.ActiveNet {
	return &nettypes.ActiveNet{
		ConfBytes: nil,
		Conf: &nettypes.NetConf{
			NetConf: cnitypes.NetConf{
				Name: netName,
				Type: netType,
			},
		},
		Runtime: &netinfo.NetInfo{
			NetName:  netName,
			ConfPath: filename,
		},
	}
}

func getValidActiveNetwork(filename, netName, netType string) *nettypes.ActiveNet {
	an := getActiveNetwork(filename, netName, netType)
	an.ConfBytes = []byte(getJSONConfig(netName, netType))
	return an
}

func getJSONConfig(netName, netType string) string {
	return fmt.Sprintf(`{"name": %q, "type": %q}`, netName, netType)
}

func getSingleNetwork(an *nettypes.ActiveNet) Networks {
	return getNetworks(an)
}

func getNetworks(ans ...*nettypes.ActiveNet) Networks {
	var ordered []*nettypes.ActiveNet
	byName := make(map[string]*nettypes.ActiveNet, len(ans))
	if len(ans) > 0 {
		ordered = append(ordered, ans...)
		sort.Sort(activeNetsSortableByPath(ordered))
		for _, an := range ans {
			byName[an.Conf.Name] = an
		}
	}
	return Networks{
		Ordered: ordered,
		ByName:  byName,
	}
}

func generateOldNetworkConfig(t *testing.T, tmp string, i int, ncfs ...*netConfigFile) string {
	cfgDir := filepath.Join(tmp, strconv.Itoa(i))
	netDir := getOldNetDir(cfgDir)
	if err := os.MkdirAll(netDir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, ncf := range ncfs {
		cfgPath := filepath.Join(netDir, ncf.filename)
		if err := ioutil.WriteFile(cfgPath, []byte(ncf.contents), 0755); err != nil {
			t.Fatal(err)
		}
	}
	return cfgDir
}

func getOldNetDir(cfgDir string) string {
	return filepath.Join(cfgDir, "net.d")
}

func getTmpDir(t *testing.T, prefix string) string {
	tmp, err := ioutil.TempDir("", prefix)
	if err != nil {
		t.Fatalf("failed to create a temporary directory: %v", err)
	}
	return tmp
}
