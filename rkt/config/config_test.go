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

package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/coreos/rkt/common"
)

const (
	CDB = common.DeprecatedCDB
)

type cfgFile struct {
	directory string
	filename  string
	contents  string
}

func TestAuthConfigFormat(t *testing.T) {
	tmp := getTmpDir(t, "rkt-auth-config-format")
	defer os.RemoveAll(tmp)
	tests := []struct {
		contents string
		expected map[string]http.Header
		fail     bool
	}{
		{"bogus contents", nil, true},
		{`{"bogus": {"foo": "bar"}}`, nil, true},
		{`{"rktKind": "foo"}`, nil, true},
		{`{"rktKind": "auth", "rktVersion": "foo"}`, nil, true},
		{`{"rktKind": "auth", "rktVersion": "v1"}`, nil, true},
		{`{"rktKind": "auth", "rktVersion": "v1", "domains": "foo"}`, nil, true},
		{`{"rktKind": "auth", "rktVersion": "v1", "domains": []}`, nil, true},
		{`{"rktKind": "auth", "rktVersion": "v1", "domains": ["coreos.com"]}`, nil, true},
		{`{"rktKind": "auth", "rktVersion": "v1", "domains": ["coreos.com"], "type": "foo"}`, nil, true},
		{`{"rktKind": "auth", "rktVersion": "v1", "domains": ["coreos.com"], "type": "basic"}`, nil, true},
		{`{"rktKind": "auth", "rktVersion": "v1", "domains": ["coreos.com"], "type": "basic", "credentials": {}}`, nil, true},
		{`{"rktKind": "auth", "rktVersion": "v1", "domains": ["coreos.com"], "type": "basic", "credentials": {"user": ""}}`, nil, true},
		{`{"rktKind": "auth", "rktVersion": "v1", "domains": ["coreos.com"], "type": "basic", "credentials": {"user": "bar"}}`, nil, true},
		{`{"rktKind": "auth", "rktVersion": "v1", "domains": ["coreos.com"], "type": "basic", "credentials": {"user": "bar", "password": ""}}`, nil, true},
		{`{"rktKind": "auth", "rktVersion": "v1", "domains": ["coreos.com"], "type": "basic", "credentials": {"user": "bar", "password": "baz"}}`, map[string]http.Header{"coreos.com": {"Authorization": []string{"Basic YmFyOmJheg=="}}}, false},
		{`{"rktKind": "auth", "rktVersion": "v1", "domains": ["coreos.com"], "type": "oauth"}`, nil, true},
		{`{"rktKind": "auth", "rktVersion": "v1", "domains": ["coreos.com"], "type": "oauth", "credentials": {}}`, nil, true},
		{`{"rktKind": "auth", "rktVersion": "v1", "domains": ["coreos.com"], "type": "oauth", "credentials": {"token": ""}}`, nil, true},
		{`{"rktKind": "auth", "rktVersion": "v1", "domains": ["coreos.com"], "type": "oauth", "credentials": {"token": "sometoken"}}`, map[string]http.Header{"coreos.com": {"Authorization": []string{"Bearer sometoken"}}}, false},
	}
	for idx, tt := range tests {
		top := getTopdir(tmp, idx)
		file := &cfgFile{
			directory: filepath.Join(top, CDB, "auth.d"),
			filename:  "cfg.json",
			contents:  tt.contents,
		}
		cfg, err := getConfigFromContents(t, []string{top}, file)
		if vErr := verifyFailure(tt.fail, tt.contents, err); vErr != nil {
			t.Errorf("%v", vErr)
		} else if !tt.fail {
			result := make(map[string]http.Header)
			for k, v := range cfg.AuthPerHost {
				result[k] = v.Header()
			}
			if !reflect.DeepEqual(result, tt.expected) {
				t.Error("Got unexpected results\nResult:\n", result, "\n\nExpected:\n", tt.expected)
			}
		}

		if _, err := json.Marshal(cfg); err != nil {
			t.Errorf("error marshaling config %v", err)
		}
	}
}

func TestAuthConfigMerge(t *testing.T) {
	tmp := getTmpDir(t, "rkt-auth-config-merge")
	defer os.RemoveAll(tmp)
	top0 := getTopdir(tmp, 0)
	top1 := getTopdir(tmp, 1)
	dir0 := filepath.Join(top0, CDB, "auth.d")
	dir1 := filepath.Join(top1, CDB, "auth.d")
	files := []*cfgFile{
		{
			directory: dir0,
			filename:  "coreos.json",
			contents:  `{"rktKind": "auth", "rktVersion": "v1", "domains": ["coreos.com"], "type": "basic", "credentials": {"user": "bar", "password": "baz"}}`,
		},
		{
			directory: dir0,
			filename:  "google.json",
			contents:  `{"rktKind": "auth", "rktVersion": "v1", "domains": ["google.com"], "type": "basic", "credentials": {"user": "foo", "password": "quux"}}`,
		},
		{
			directory: dir1,
			filename:  "google-overridden.json",
			contents:  `{"rktKind": "auth", "rktVersion": "v1", "domains": ["google.com"], "type": "oauth", "credentials": {"token": "google-token"}}`,
		},
		{
			directory: dir1,
			filename:  "quay.json",
			contents:  `{"rktKind": "auth", "rktVersion": "v1", "domains": ["quay.io"], "type": "oauth", "credentials": {"token": "quay-token"}}`,
		},
	}
	expectedCreds := map[string]http.Header{
		"coreos.com": {"Authorization": []string{"Basic YmFyOmJheg=="}},
		"google.com": {"Authorization": []string{"Bearer google-token"}},
		"quay.io":    {"Authorization": []string{"Bearer quay-token"}},
	}
	cfg, err := getConfigFromContents(t, []string{top0, top1}, files...)
	if err != nil {
		t.Fatal(err)
	}
	got := ResolveAuthPerHost(cfg.AuthPerHost)
	for host, headers := range got {
		if ex, ok := expectedCreds[host]; ok {
			delete(expectedCreds, host)
			if !reflect.DeepEqual(ex, headers) {
				t.Errorf("expected headers for host %q:\n%#v\ngot:\n%#v", host, ex, headers)
			}
		} else {
			t.Errorf("got unexpected headers for host %q: %#v", host, headers)
		}
	}
	for host, headers := range expectedCreds {
		t.Errorf("did not get headers for host %q: %#v", host, headers)
	}
}

func TestDockerAuthConfigFormat(t *testing.T) {
	tmp := getTmpDir(t, "rkt-docker-auth-config-format")
	defer os.RemoveAll(tmp)
	tests := []struct {
		contents string
		expected map[string]BasicCredentials
		fail     bool
	}{
		{"bogus contents", nil, true},
		{`{"bogus": {"foo": "bar"}}`, nil, true},
		{`{"rktKind": "foo"}`, nil, true},
		{`{"rktKind": "dockerAuth", "rktVersion": "foo"}`, nil, true},
		{`{"rktKind": "dockerAuth", "rktVersion": "v1"}`, nil, true},
		{`{"rktKind": "dockerAuth", "rktVersion": "v1", "registries": "foo"}`, nil, true},
		{`{"rktKind": "dockerAuth", "rktVersion": "v1", "registries": []}`, nil, true},
		{`{"rktKind": "dockerAuth", "rktVersion": "v1", "registries": ["coreos.com"]}`, nil, true},
		{`{"rktKind": "dockerAuth", "rktVersion": "v1", "registries": ["coreos.com"], "credentials": {}}`, nil, true},
		{`{"rktKind": "dockerAuth", "rktVersion": "v1", "registries": ["coreos.com"], "credentials": {"user": ""}}`, nil, true},
		{`{"rktKind": "dockerAuth", "rktVersion": "v1", "registries": ["coreos.com"], "credentials": {"user": "bar"}}`, nil, true},
		{`{"rktKind": "dockerAuth", "rktVersion": "v1", "registries": ["coreos.com"], "credentials": {"user": "bar", "password": ""}}`, nil, true},
		{`{"rktKind": "dockerAuth", "rktVersion": "v1", "registries": ["coreos.com"], "credentials": {"user": "bar", "password": "baz"}}`, map[string]BasicCredentials{"coreos.com": BasicCredentials{User: "bar", Password: "baz"}}, false},
	}
	for idx, tt := range tests {
		top := getTopdir(tmp, idx)
		file := &cfgFile{
			directory: filepath.Join(top, CDB, "auth.d"),
			filename:  "cfg.json",
			contents:  tt.contents,
		}
		cfg, err := getConfigFromContents(t, []string{top}, file)
		if vErr := verifyFailure(tt.fail, tt.contents, err); vErr != nil {
			t.Errorf("%v", vErr)
		} else if !tt.fail {
			result := cfg.DockerCredentialsPerRegistry
			if !reflect.DeepEqual(result, tt.expected) {
				t.Error("Got unexpected results\nResult:\n", result, "\n\nExpected:\n", tt.expected)
			}
		}

		if _, err := json.Marshal(cfg); err != nil {
			t.Errorf("error marshaling config %v", err)
		}
	}
}

func TestDockerAuthConfigMerge(t *testing.T) {
	tmp := getTmpDir(t, "rkt-docker-auth-config-merge")
	defer os.RemoveAll(tmp)
	top0 := getTopdir(tmp, 0)
	top1 := getTopdir(tmp, 1)
	dir0 := filepath.Join(top0, CDB, "auth.d")
	dir1 := filepath.Join(top1, CDB, "auth.d")
	files := []*cfgFile{
		{
			directory: dir0,
			filename:  "coreos.json",
			contents:  `{"rktKind": "dockerAuth", "rktVersion": "v1", "registries": ["coreos.com"], "credentials": {"user": "bar", "password": "baz"}}`,
		},
		{
			directory: dir0,
			filename:  "google.json",
			contents:  `{"rktKind": "dockerAuth", "rktVersion": "v1", "registries": ["google.com"], "credentials": {"user": "foo", "password": "quux"}}`,
		},
		{
			directory: dir1,
			filename:  "google-overridden.json",
			contents:  `{"rktKind": "dockerAuth", "rktVersion": "v1", "registries": ["google.com"], "credentials": {"user": "goo", "password": "gle"}}`,
		},
		{
			directory: dir1,
			filename:  "quay.json",
			contents:  `{"rktKind": "dockerAuth", "rktVersion": "v1", "registries": ["quay.io"], "credentials": {"user": "qu", "password": "ay"}}`,
		},
	}
	expectedCreds := map[string]BasicCredentials{
		"coreos.com": BasicCredentials{User: "bar", Password: "baz"},
		"google.com": BasicCredentials{User: "goo", Password: "gle"},
		"quay.io":    BasicCredentials{User: "qu", Password: "ay"},
	}
	cfg, err := getConfigFromContents(t, []string{top0, top1}, files...)
	if err != nil {
		t.Fatal(err)
	}
	got := cfg.DockerCredentialsPerRegistry
	for registry, creds := range got {
		if ex, ok := expectedCreds[registry]; ok {
			delete(expectedCreds, registry)
			if !reflect.DeepEqual(ex, creds) {
				t.Errorf("expected credentials for registry %q:\n%#v\ngot:\n%#v", registry, ex, creds)
			}
		} else {
			t.Errorf("got unexpected credentials for registry %q: %#v", registry, creds)
		}
	}
	for registry, creds := range expectedCreds {
		t.Errorf("did not get credentials for registry %q: %#v", registry, creds)
	}
}

func TestPathsConfigFormat(t *testing.T) {
	tmp := getTmpDir(t, "rkt-paths-config-format")
	defer os.RemoveAll(tmp)
	tests := []struct {
		contents string
		expected ConfigurablePaths
		fail     bool
	}{
		{"bogus contents", ConfigurablePaths{}, true},
		{`{"bogus": {"foo": "bar"}}`, ConfigurablePaths{}, true},
		{`{"rktKind": "foo"}`, ConfigurablePaths{}, true},
		{`{"rktKind": "paths", "rktVersion": "foo"}`, ConfigurablePaths{}, true},
		{`{"rktKind": "paths", "rktVersion": "v1"}`, ConfigurablePaths{}, false},
		{`{"rktKind": "paths", "rktVersion": "v1", "data": "rel/path"}`, ConfigurablePaths{}, true},
		{`{"rktKind": "paths", "rktVersion": "v1", "data": "/abs/path"}`, ConfigurablePaths{DataDir: "/abs/path"}, false},
		{`{"rktKind": "paths", "rktVersion": "v1", "data": "/abs/path1", "stage1-images": "rel/path"}`, ConfigurablePaths{}, true},
		{`{"rktKind": "paths", "rktVersion": "v1", "data": "/abs/path1", "stage1-images": "/abs/path2"}`, ConfigurablePaths{DataDir: "/abs/path1", Stage1ImagesDir: "/abs/path2"}, false},
	}
	for idx, tt := range tests {
		top := getTopdir(tmp, idx)
		file := &cfgFile{
			directory: filepath.Join(top, CDB, "paths.d"),
			filename:  "cfg.json",
			contents:  tt.contents,
		}
		cfg, err := getConfigFromContents(t, []string{top}, file)
		if vErr := verifyFailure(tt.fail, tt.contents, err); vErr != nil {
			t.Errorf("%v", vErr)
		} else if !tt.fail {
			result := cfg.Paths
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("Got unexpected results\nResult:\n%#v\n\nExpected:\n%#v", result, tt.expected)
			}
		}
	}
}

func TestPathsConfigMerge(t *testing.T) {
	tmp := getTmpDir(t, "rkt-paths-config-merge")
	defer os.RemoveAll(tmp)
	top0 := getTopdir(tmp, 0)
	top1 := getTopdir(tmp, 1)
	dir0 := filepath.Join(top0, CDB, "paths.d")
	dir1 := filepath.Join(top1, CDB, "paths.d")
	files := []*cfgFile{
		{
			directory: dir0,
			filename:  "coreos.json",
			contents:  `{"rktKind": "paths", "rktVersion": "v1", "data": "/abs/path", "stage1-images": "/abs/path2"}`,
		},
		{
			directory: dir1,
			filename:  "quay.json",
			contents:  `{"rktKind": "paths", "rktVersion": "v1", "data": "/new/abs/path", "stage1-images": "/new/abs/path2"}`,
		},
	}
	expectedCreds := ConfigurablePaths{
		DataDir:         "/new/abs/path",
		Stage1ImagesDir: "/new/abs/path2",
	}
	cfg, err := getConfigFromContents(t, []string{top0, top1}, files...)
	if err != nil {
		t.Fatal(err)
	}
	got := cfg.Paths
	if !reflect.DeepEqual(expectedCreds, got) {
		t.Errorf("expected paths:\n%#v\ngot:\n%#v", expectedCreds, got)
	}
}

func TestStage1ConfigFormat(t *testing.T) {
	tmp := getTmpDir(t, "rkt-stage1-config-format")
	defer os.RemoveAll(tmp)
	tests := []struct {
		contents string
		expected Stage1Data
		fail     bool
	}{
		{"bogus contents", Stage1Data{}, true},
		{`{"bogus": {"foo": "bar"}}`, Stage1Data{}, true},
		{`{"rktKind": "foo"}`, Stage1Data{}, true},
		{`{"rktKind": "stage1", "rktVersion": "foo"}`, Stage1Data{}, true},
		{`{"rktKind": "stage1", "rktVersion": "v1"}`, Stage1Data{}, false},
		{`{"rktKind": "stage1", "rktVersion": "v1", "name": "example.com/stage1"}`, Stage1Data{}, true},
		{`{"rktKind": "stage1", "rktVersion": "v1", "version": "1.2.3"}`, Stage1Data{}, true},
		{`{"rktKind": "stage1", "rktVersion": "v1", "name": "example.com/stage1", "version": "1.2.3"}`, Stage1Data{Name: "example.com/stage1", Version: "1.2.3"}, false},
		{`{"rktKind": "stage1", "rktVersion": "v1", "location": "/image.aci"}`, Stage1Data{Location: "/image.aci"}, false},
		{`{"rktKind": "stage1", "rktVersion": "v1", "name": "example.com/stage1", "location": "/image.aci"}`, Stage1Data{}, true},
		{`{"rktKind": "stage1", "rktVersion": "v1", "version": "1.2.3", "location": "/image.aci"}`, Stage1Data{}, true},
		{`{"rktKind": "stage1", "rktVersion": "v1", "name": "example.com/stage1", "version": "1.2.3", "location": "/image.aci"}`, Stage1Data{Name: "example.com/stage1", Version: "1.2.3", Location: "/image.aci"}, false},
	}
	for idx, tt := range tests {
		top := getTopdir(tmp, idx)
		file := &cfgFile{
			directory: filepath.Join(top, CDB, "stage1.d"),
			filename:  "cfg.json",
			contents:  tt.contents,
		}
		cfg, err := getConfigFromContents(t, []string{top}, file)
		if vErr := verifyFailure(tt.fail, tt.contents, err); vErr != nil {
			t.Errorf("%v", vErr)
		} else if !tt.fail {
			result := cfg.Stage1
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("Got unexpected results\nResult:\n%#v\n\nExpected:\n%#v", result, tt.expected)
			}
		}
	}
}

func TestStage1ConfigMerge(t *testing.T) {
	tmp := getTmpDir(t, "rkt-stage1-config-merge")
	defer os.RemoveAll(tmp)
	top0 := getTopdir(tmp, 0)
	top1 := getTopdir(tmp, 1)
	dir0 := filepath.Join(top0, CDB, "stage1.d")
	dir1 := filepath.Join(top1, CDB, "stage1.d")
	files := []*cfgFile{
		{
			directory: dir0,
			filename:  "coreos.json",
			contents:  `{"rktKind": "stage1", "rktVersion": "v1", "name": "coreos.com/rkt/stage1-something", "version": "0.0.1", "location": "https://coreos.com/some/url/stage1.aci"}`,
		},
		{
			directory: dir1,
			filename:  "name-and-version-overridden.json",
			contents:  `{"rktKind": "stage1", "rktVersion": "v1", "name": "coreos.com/rkt/stage1-better", "version": "1.2.3"}`,
		},
		{
			directory: dir1,
			filename:  "location-overridden.json",
			contents:  `{"rktKind": "stage1", "rktVersion": "v1", "location": "https://coreos.com/some/url/better.aci"}`,
		},
	}
	expectedStage1 := Stage1Data{
		Name:     "coreos.com/rkt/stage1-better",
		Version:  "1.2.3",
		Location: "https://coreos.com/some/url/better.aci",
	}
	cfg, err := getConfigFromContents(t, []string{top0, top1}, files...)
	if err != nil {
		t.Fatal(err)
	}
	got := cfg.Stage1
	if !reflect.DeepEqual(expectedStage1, got) {
		t.Errorf("expected stage1:\n%#v\ngot:\n%#v", expectedStage1, got)
	}
}

func TestPreferNewConfigToOld(t *testing.T) {
	tmp := getTmpDir(t, "rkt-backward-compat-config")
	defer os.RemoveAll(tmp)
	top := getTopdir(tmp, 0)
	cfgDir := filepath.Join(top, common.Stage0CDB)
	files := []*cfgFile{
		{
			directory: filepath.Join(cfgDir, "stage1.d"),
			filename:  "coreos.json",
			contents:  `{"rktKind": "stage1", "rktVersion": "v1", "name": "coreos.com/rkt/stage1-better", "version": "1.2.3", "location": "https://coreos.com/some/url/better.aci"}`,
		},
		{
			directory: filepath.Join(top, "paths.d"),
			filename:  "coreos.json",
			contents:  `{"rktKind": "paths", "rktVersion": "v1", "data": "/abs/path", "stage1-images": "/abs/path2"}`,
		},
	}
	expectedStage1 := Stage1Data{
		Name:     "coreos.com/rkt/stage1-better",
		Version:  "1.2.3",
		Location: "https://coreos.com/some/url/better.aci",
	}
	expectedPaths := ConfigurablePaths{}
	cfg, err := getConfigFromContents(t, []string{top}, files...)
	if err != nil {
		t.Fatal(err)
	}
	gotStage1 := cfg.Stage1
	if !reflect.DeepEqual(expectedStage1, gotStage1) {
		t.Errorf("expected stage1:\n%#v\ngot:\n%#v", expectedStage1, gotStage1)
	}
	gotPaths := cfg.Paths
	if !reflect.DeepEqual(expectedPaths, gotPaths) {
		t.Errorf("expected paths:\n%#v\ngot:\n%#v", expectedPaths, gotPaths)
	}
}

func TestNewConfigDirectory(t *testing.T) {
	tmp := getTmpDir(t, "rkt-new-config-directory")
	defer os.RemoveAll(tmp)
	top := getTopdir(tmp, 0)
	cfgDir := filepath.Join(top, common.Stage0CDB)
	files := []*cfgFile{
		{
			directory: filepath.Join(cfgDir, "stage1.d"),
			filename:  "coreos.json",
			contents:  `{"rktKind": "stage1", "rktVersion": "v1", "name": "coreos.com/rkt/stage1-better", "version": "1.2.3", "location": "https://coreos.com/some/url/better.aci"}`,
		},
	}
	expectedStage1 := Stage1Data{
		Name:     "coreos.com/rkt/stage1-better",
		Version:  "1.2.3",
		Location: "https://coreos.com/some/url/better.aci",
	}
	cfg, err := getConfigFromContents(t, []string{top}, files...)
	if err != nil {
		t.Fatal(err)
	}
	gotStage1 := cfg.Stage1
	if !reflect.DeepEqual(expectedStage1, gotStage1) {
		t.Errorf("expected stage1:\n%#v\ngot:\n%#v", expectedStage1, gotStage1)
	}
}

func getTmpDir(t *testing.T, prefix string) string {
	tmp, err := ioutil.TempDir("", prefix)
	if err != nil {
		t.Fatalf("failed to create a temporary directory: %v", err)
	}
	return tmp
}

func getTopdir(tmp string, idx int) string {
	return filepath.Join(tmp, fmt.Sprintf("testdir-%d", idx))
}

func getConfigFromContents(t *testing.T, topdirs []string, files ...*cfgFile) (*Config, error) {
	for _, file := range files {
		if err := os.MkdirAll(file.directory, 0755); err != nil {
			return nil, err
		}
		f, err := os.Create(filepath.Join(file.directory, file.filename))
		if err != nil {
			return nil, err
		}
		// only closing, it will be removed together with the tmp directory
		defer f.Close()
		if _, err := f.Write([]byte(file.contents)); err != nil {
			return nil, err
		}
	}
	return GetConfigFrom(topdirs...)
}

func verifyFailure(shouldFail bool, contents string, err error) error {
	var vErr error = nil
	if err != nil {
		if !shouldFail {
			vErr = fmt.Errorf("Expected test to succeed, failed unexpectedly (contents: `%s`): %v", contents, err)
		}
	} else if shouldFail {
		vErr = fmt.Errorf("Expected test to fail, succeeded unexpectedly (contents: `%s`)", contents)
	}
	return vErr
}
