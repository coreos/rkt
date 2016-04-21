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
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type testNoopParser struct{}

func (*testNoopParser) Parse(idx *PathIndex, raw []byte) error {
	return nil
}

type failParser struct{}

func (p *failParser) Parse(idx *PathIndex, raw []byte) error {
	return errors.New("born to fail")
}

type dirWithLines struct {
	dir   string
	lines []string
}

type testParser struct {
	linesPerDir map[int]*dirWithLines
}

func newTestParser() *testParser {
	return &testParser{
		linesPerDir: make(map[int]*dirWithLines),
	}
}

func (p *testParser) reset() {
	p.linesPerDir = make(map[int]*dirWithLines)
}

func (p *testParser) Parse(pi *PathIndex, raw []byte) error {
	s := getScanner(raw)
	_, _, err := getKindAndVersion(s)
	if err != nil {
		return err
	}
	var lines []string
	for s.Scan() {
		lines = append(lines, s.Text())
	}
	if err := s.Err(); err != nil {
		return err
	}
	dwl, ok := p.linesPerDir[pi.Index]
	if !ok {
		dwl = &dirWithLines{
			dir: pi.Path,
		}
		p.linesPerDir[pi.Index] = dwl
	}
	dwl.lines = append(dwl.lines, lines...)
	return nil
}

type testConfigType struct{}

func (*testConfigType) Extension() string {
	return "test"
}

func (*testConfigType) GetKindAndVersion(raw []byte) (string, string, error) {
	s := getScanner(raw)
	return getKindAndVersion(s)
}

func getScanner(raw []byte) *bufio.Scanner {
	return bufio.NewScanner(bytes.NewReader(raw))
}

func getKindAndVersion(s *bufio.Scanner) (string, string, error) {
	kind := ""
	version := ""
	for s.Scan() {
		switch {
		case kind == "":
			kind = s.Text()
		case version == "":
			version = s.Text()
		}
		if kind != "" && version != "" {
			break
		}
	}
	if err := s.Err(); err != nil {
		return "", "", err
	}
	if kind == "" {
		return "", "", errors.New("no kind found")
	}
	if version == "" {
		return "", "", errors.New("no version found")
	}
	return kind, version, nil
}

func TestParserRegistration(t *testing.T) {
	tests := []struct {
		desc    string
		setups  []*ParserSetup
		success bool
	}{
		{
			desc:    "no parsers registered",
			setups:  nil,
			success: true,
		},
		{
			desc: "various parsers registered",
			setups: []*ParserSetup{
				getTestParserSetup("a", "1"),
				getTestParserSetup("a", "2"),
				getTestParserSetup("b", "1"),
				getTestParserSetup("b", "2"),
			},
			success: true,
		},
		{
			desc: "parser with an empty kind",
			setups: []*ParserSetup{
				getTestParserSetup("", "1"),
			},
			success: false,
		},
		{
			desc: "parser with an empty version",
			setups: []*ParserSetup{
				getTestParserSetup("a", ""),
			},
			success: false,
		},
		{
			desc: "nil parser",
			setups: []*ParserSetup{
				getParserSetup("a", "1", nil),
			},
			success: false,
		},
		{
			desc: "duplicated parser",
			setups: []*ParserSetup{
				getTestParserSetup("a", "1"),
				getTestParserSetup("a", "1"),
			},
			success: false,
		},
	}
	for i, tt := range tests {
		t.Logf("Test #%d: %s", i, tt.desc)
		d := NewDirectory("foo", &testConfigType{})
		err := d.RegisterParsers(tt.setups)
		if err != nil && tt.success {
			t.Errorf("expected success, got an error: %v", err)
			continue
		}
		if err == nil && !tt.success {
			t.Error("expected failure, got no error")
			continue
		}
	}
}

func TestSubdirectoryRegistration(t *testing.T) {
	tests := []struct {
		desc     string
		setups   []*SubdirSetup
		success  bool
		expected map[string][]string
	}{
		{
			desc:     "no directories registered",
			setups:   nil,
			success:  true,
			expected: map[string][]string{},
		},
		{
			desc: "various directories registered",
			setups: []*SubdirSetup{
				getSubdirSetup("d1", "a"),
				getSubdirSetup("d2", "a", "b"),
				getSubdirSetup("d3", "b"),
			},
			success: true,
			expected: map[string][]string{
				"d1": []string{"a"},
				"d2": []string{"a", "b"},
				"d3": []string{"b"},
			},
		},
		{
			desc: "kinds appended to the registered directory",
			setups: []*SubdirSetup{
				getSubdirSetup("d1", "a"),
				getSubdirSetup("d1", "b"),
				getSubdirSetup("d1", "c"),
			},
			success: true,
			expected: map[string][]string{
				"d1": []string{"a", "b", "c"},
			},
		},
		{
			desc: "subdir with an empty dir",
			setups: []*SubdirSetup{
				getSubdirSetup("", "a"),
			},
			success:  false,
			expected: nil,
		},
		{
			desc: "subdir with no kinds",
			setups: []*SubdirSetup{
				getSubdirSetup("d1"),
			},
			success:  false,
			expected: nil,
		},
	}
	for i, tt := range tests {
		t.Logf("Test #%d: %s", i, tt.desc)
		d := NewDirectory("foo", &testConfigType{})
		err := d.RegisterSubdirectories(tt.setups)
		if err != nil && tt.success {
			t.Errorf("expected success, got an error: %v", err)
			continue
		}
		if err == nil && !tt.success {
			t.Error("expected failure, got no error")
			continue
		}
		if tt.success && !reflect.DeepEqual(tt.expected, d.configSubdirs) {
			t.Errorf("expected subdirs:\n%#v\ngot:\n%#v\n", tt.expected, d.configSubdirs)
			continue
		}
	}
}

func TestInvalidDirectories(t *testing.T) {
	tmp := getTempDir(t, "rkt-config-invalid-directory")
	defer os.RemoveAll(tmp)
	cfg := "cfg"
	file := "notADir"
	dir := "normalDir"
	missing := "missingDir"
	{
		path := filepath.Join(tmp, cfg, dir)
		if err := os.MkdirAll(path, 0755); err != nil {
			t.Fatalf("failed to create a directory %q: %v", path, err)
		}
	}
	for _, f := range []string{filepath.Join(tmp, file), filepath.Join(tmp, cfg, file)} {
		if err := ioutil.WriteFile(f, nil, 0644); err != nil {
			t.Fatalf("failed to create a file %q: %v", f, err)
		}
	}
	skipReason := ""
	if u, err := user.Current(); err != nil || u.Username == "root" {
		skipReason = "probably running the test as root, so we can access every file/directory"
	}
	if skipReason == "" {
		if _, err := os.Stat(filepath.Join("/root", dir)); err == nil || os.IsNotExist(err) {
			skipReason = "/root is either missing or its contents are accessible"
		}
	}

	tests := []struct {
		desc    string
		topdir  string
		dir     string
		subdir  string
		success bool
		skip    string
	}{
		{
			desc:    "dir and subdir exist",
			topdir:  tmp,
			dir:     cfg,
			subdir:  dir,
			success: true,
		},
		{
			desc:    "dir exists, subdir is missing",
			topdir:  tmp,
			dir:     cfg,
			subdir:  missing,
			success: true,
		},
		{
			desc:    "dir exists, subdir is a file",
			topdir:  tmp,
			dir:     cfg,
			subdir:  file,
			success: false,
		},
		{
			desc:    "dir is missing",
			topdir:  tmp,
			dir:     missing,
			subdir:  dir,
			success: true,
		},
		{
			desc:    "dir is a file",
			topdir:  tmp,
			dir:     file,
			subdir:  dir,
			success: false,
		},
		{
			desc:    "dir exists, no permissions for subdir",
			topdir:  "/",
			dir:     "root",
			subdir:  dir,
			success: false,
			skip:    skipReason,
		},
		{
			desc:    "no permissions for dir",
			topdir:  "/root",
			dir:     dir,
			subdir:  dir,
			success: false,
			skip:    skipReason,
		},
	}

	for i, tt := range tests {
		t.Logf("Test #%d: %s", i, tt.desc)
		t.Logf("  Config directory: %s", filepath.Join(tt.topdir, tt.dir))
		t.Logf("  Config subdirectory: %s", tt.subdir)
		if tt.skip != "" {
			t.Logf("skipped, reason: %s", tt.skip)
		}
		d := NewDirectory(tt.dir, &testConfigType{})
		kind := "a"
		err := d.RegisterSubdirectory(tt.subdir, []string{kind})
		if err != nil {
			t.Errorf("unexpected error when registering a subdirectory %q for directory %q and kind %q: %v", tt.subdir, tt.dir, kind, err)
			continue
		}
		err = d.WalkDirectories(tt.topdir)
		if err != nil && tt.success {
			t.Errorf("expected success, got an error: %v", err)
			continue
		}
		if err == nil && !tt.success {
			t.Error("expected failure, got no error")
			continue
		}
	}
}

// dir
// [file]
// <symlink>
//
// tmp - /tmp/rkt-config-validXXXXXX
// + testX - a per-test directory, toplevel directory of a tree described by "layout" in test struct
// | + etc - "dirs" in test struct will denote these directories to be visited
// | | + cfg1 - this is passed to NewDirectory function
// | | | + subdir1 - this is passed to RegisterSubdirectory
// | | | | + [cfg.test] - this might be parsed
// | | | | + subsubdir - this directory will be ignored
// | | | | | + [file]
// | | | | + <link> - this symlink will be ignored
// | | | + subdir2
// | | + cfg2
// | + lib
// + testY
func TestValidConfig(t *testing.T) {
	tmp := getTempDir(t, "rkt-config-valid")
	defer os.RemoveAll(tmp)
	parser := newTestParser()

	tests := []struct {
		desc          string
		topdir        string
		cfgdir        string
		dirs          []string
		psetup        []*ParserSetup
		ssetup        []*SubdirSetup
		layout        *testDir
		success       bool
		linesPerIndex [][]string
	}{
		{
			desc:   "ignore symlinks, files without valid extension, and files in lower directories",
			topdir: tmp,
			dirs:   []string{"etc"},
			cfgdir: "cfg",
			psetup: []*ParserSetup{
				{
					Kind:    "a",
					Version: "1",
					Parser:  parser,
				},
			},
			ssetup: []*SubdirSetup{
				{
					Subdir: "stuff",
					Kinds:  []string{"a"},
				},
			},
			layout: dirsWithTestDir(&testDir{
				symlinks: map[string]string{
					"somefile": "symlink",
				},
				files: map[string]string{
					"somefile.foo": getValidContents("a", "1", "fail: parsed file with wrong extension"),
					"cfg.test":     getValidContents("a", "1", "rkt"),
				},
				dirs: map[string]*testDir{
					"ignorethisdir": &testDir{
						files: map[string]string{
							"ignoredcfg.test": getValidContents("a", "1", "fail: parsed file inside a subsubdirectory (ought to be ignored)"),
						},
					},
				},
			}, "etc", "cfg", "stuff"),
			success:       true,
			linesPerIndex: [][]string{[]string{"rkt"}},
		},

		{
			desc:   "fail on invalid config file in directory (missing kind or version)",
			topdir: tmp,
			dirs:   []string{"etc"},
			cfgdir: "cfg",
			psetup: []*ParserSetup{
				{
					Kind:    "a",
					Version: "1",
					Parser:  parser,
				},
			},
			ssetup: []*SubdirSetup{
				{
					Subdir: "stuff",
					Kinds:  []string{"a"},
				},
			},
			layout: dirsWithTestDir(&testDir{
				files: map[string]string{
					"cfg.test": "lalalala\n",
				},
			}, "etc", "cfg", "stuff"),
			success: false,
		},

		{
			desc:   "fail on wrong kind for a directory",
			topdir: tmp,
			dirs:   []string{"etc"},
			cfgdir: "cfg",
			psetup: []*ParserSetup{
				{
					Kind:    "a",
					Version: "1",
					Parser:  parser,
				},
			},
			ssetup: []*SubdirSetup{
				{
					Subdir: "stuff",
					Kinds:  []string{"a"},
				},
			},
			layout: dirsWithTestDir(&testDir{
				files: map[string]string{
					"cfg.test": getValidContents("b", "1", "rkt"),
				},
			}, "etc", "cfg", "stuff"),
			success: false,
		},

		{
			desc:   "fail when parser for given kind is missing",
			topdir: tmp,
			dirs:   []string{"etc"},
			cfgdir: "cfg",
			psetup: nil,
			ssetup: []*SubdirSetup{
				{
					Subdir: "stuff",
					Kinds:  []string{"a"},
				},
			},
			layout: dirsWithTestDir(&testDir{
				files: map[string]string{
					"cfg.test": getValidContents("a", "1", "rkt"),
				},
			}, "etc", "cfg", "stuff"),
			success: false,
		},

		{
			desc:   "fail when parser for given kind is missing",
			topdir: tmp,
			dirs:   []string{"etc"},
			cfgdir: "cfg",
			psetup: []*ParserSetup{
				{
					Kind:    "a",
					Version: "1",
					Parser:  parser,
				},
			},
			ssetup: []*SubdirSetup{
				{
					Subdir: "stuff",
					Kinds:  []string{"a"},
				},
			},
			layout: dirsWithTestDir(&testDir{
				files: map[string]string{
					"cfg.test": getValidContents("a", "2", "rkt"),
				},
			}, "etc", "cfg", "stuff"),
			success: false,
		},

		{
			desc:   "propagate parser failure",
			topdir: tmp,
			dirs:   []string{"etc"},
			cfgdir: "cfg",
			psetup: []*ParserSetup{
				{
					Kind:    "a",
					Version: "1",
					Parser:  &failParser{},
				},
			},
			ssetup: []*SubdirSetup{
				{
					Subdir: "stuff",
					Kinds:  []string{"a"},
				},
			},
			layout: dirsWithTestDir(&testDir{
				files: map[string]string{
					"cfg.test": getValidContents("a", "1", "rkt"),
				},
			}, "etc", "cfg", "stuff"),
			success: false,
		},

		{
			desc:   "ignores unregistered directories",
			topdir: tmp,
			dirs:   []string{"etc"},
			cfgdir: "cfg",
			psetup: []*ParserSetup{
				{
					Kind:    "a",
					Version: "1",
					Parser:  parser,
				},
			},
			ssetup: []*SubdirSetup{
				{
					Subdir: "stuff",
					Kinds:  []string{"a"},
				},
			},
			layout: dirsWithTestDir(&testDir{
				dirs: map[string]*testDir{
					"stuff": &testDir{
						files: map[string]string{
							"cfg.test": getValidContents("a", "1", "rkt", "foo"),
						},
					},
					"unregistered": &testDir{
						files: map[string]string{
							"cfg.test": getValidContents("a", "1", "bar", "fail: File in an unregistered directory was not ignored as it should"),
						},
					},
				},
			}, "etc", "cfg"),
			success:       true,
			linesPerIndex: [][]string{[]string{"rkt", "foo"}},
		},

		// tmp
		// + testX
		//   + etc
		//   | + cfg
		//   |   + aaa
		//   |   | + [cfg.test] (has ETC AAA A)
		//   |   + bbb
		//   |   | + [cfg.test] (has ETC BBB B)
		//   |   + common
		//   |     + [a.test] (has ETC COMMON A)
		//   |     + [b.test] (has ETC COMMON B)
		//   + lib
		//   | + cfg
		//   |   + aaa
		//   |   | + [cfg.test] (has LIB AAA A)
		//   |   + bbb
		//   |   | + [cfg.test] (has LIB BBB B)
		//   |   + common
		//   |     + [a.test] (has LIB COMMON A)
		//   |     + [b.test] (has LIB COMMON B)
		//   + user
		//     + cfg
		//       + aaa
		//       | + [cfg.test] (has USER AAA A)
		//       + bbb
		//       | + [cfg.test] (has USER BBB B)
		//       + common
		//         + [a.test] (has USER COMMON A)
		//         + [b.test] (has USER COMMON B)
		{
			desc:   "multiple directories work",
			topdir: tmp,
			dirs:   []string{"etc", "lib", "user"},
			cfgdir: "cfg",
			psetup: []*ParserSetup{
				{
					Kind:    "a",
					Version: "1",
					Parser:  parser,
				},
				{
					Kind:    "b",
					Version: "2",
					Parser:  parser,
				},
			},
			ssetup: []*SubdirSetup{
				{
					Subdir: "aaa",
					Kinds:  []string{"a"},
				},
				{
					Subdir: "bbb",
					Kinds:  []string{"b"},
				},
				{
					Subdir: "common",
					Kinds:  []string{"a", "b"},
				},
			},
			layout: &testDir{
				dirs: getDirsForMultipleTest(),
			},
			success: true,
			linesPerIndex: [][]string{
				[]string{
					"ETC AAA A",
					"ETC BBB B",
					"ETC COMMON A",
					"ETC COMMON B",
				},
				[]string{
					"LIB AAA A",
					"LIB BBB B",
					"LIB COMMON A",
					"LIB COMMON B",
				},
				[]string{
					"USER AAA A",
					"USER BBB B",
					"USER COMMON A",
					"USER COMMON B",
				},
			},
		},
	}

	for i, tt := range tests {
		t.Logf("Test #%d: %s", i, tt.desc)
		parser.reset()
		d := NewDirectory(tt.cfgdir, &testConfigType{})
		if err := d.RegisterParsers(tt.psetup); err != nil {
			t.Errorf("unexpected error when registering parsers: %v", err)
			continue
		}
		if err := d.RegisterSubdirectories(tt.ssetup); err != nil {
			t.Errorf("unexpected error when registering subdirectories: %v", err)
			continue
		}
		testDir := filepath.Join(tmp, fmt.Sprintf("testdir%d", i))
		if err := createLayout(testDir, tt.layout); err != nil {
			t.Errorf("unexpected error when creating layout: %v", err)
			continue
		}
		walkedDirs := make([]string, 0, len(tt.dirs))
		for _, d := range tt.dirs {
			walkedDirs = append(walkedDirs, filepath.Join(testDir, d))
		}
		err := d.WalkDirectories(walkedDirs...)
		if err != nil && tt.success {
			t.Errorf("expected success, got an error: %v", err)
			continue
		}
		if err == nil && !tt.success {
			t.Error("expected failure, got no error")
			continue
		}
		if tt.success {
			for idx, dwl := range parser.linesPerDir {
				t.Logf("Checking lines from %q", dwl.dir)
				if len(tt.linesPerIndex) <= idx {
					t.Errorf("expected to parse only %d directories, but apparently we parsed %d (unplanned directory is %q)", len(tt.linesPerIndex), idx+1, dwl.dir)
					continue
				}
				expectedLinesInIndex := tt.linesPerIndex[idx]
				expectedSet := toSet(expectedLinesInIndex)
				if len(expectedSet) != len(expectedLinesInIndex) {
					t.Errorf("Please fix the test, so all the expected lines are unique")
				}
				for _, line := range dwl.lines {
					if _, ok := expectedSet[line]; !ok {
						t.Errorf("Got an unexpected line: %s", line)
					} else {
						delete(expectedSet, line)
					}
				}
				for line := range expectedSet {
					t.Errorf("Expected a line: %s", line)
				}
			}
		}
	}
}

func TestEmptyConfigDirectoryBasename(t *testing.T) {
	tmp := getTempDir(t, "rkt-empty-cdb")
	defer os.RemoveAll(tmp)
	parser := newTestParser()
	d := NewDirectory("", &testConfigType{})
	if err := d.RegisterParser("a", "1", parser); err != nil {
		t.Fatalf("unexpected error when registering a parser: %v", err)
	}
	if err := d.RegisterSubdirectory("stuff", []string{"a"}); err != nil {
		t.Fatalf("unexpected error when registering a subdirectory: %v", err)
	}
	topDirs := []string{}
	pathIndices := []*PathIndex{}
	for idx, bn := range []string{"1", "2"} {
		top := filepath.Join(tmp, bn)
		testDir := filepath.Join(top, "stuff")
		if err := os.MkdirAll(testDir, 0755); err != nil {
			t.Fatalf("failed to create a directory: %v", err)
		}
		filename := fmt.Sprintf("%s.test", bn)
		file := filepath.Join(testDir, filename)
		if err := ioutil.WriteFile(file, []byte(getValidContents("a", "1", bn)), 0755); err != nil {
			t.Fatalf("failed to write a file: %v", err)
		}
		topDirs = append(topDirs, top)
		pathIndex := &PathIndex{
			Index:        idx,
			Path:         top,
			Subdirectory: bn,
			Filename:     filename,
		}
		pathIndices = append(pathIndices, pathIndex)
	}
	if err := d.WalkDirectories(topDirs...); err != nil {
		t.Fatalf("expected success, got an error: %v", err)
	}
	expected := map[int]*dirWithLines{
		0: &dirWithLines{
			dir:   topDirs[0],
			lines: []string{"1"},
		},
		1: &dirWithLines{
			dir:   topDirs[1],
			lines: []string{"2"},
		},
	}
	if !reflect.DeepEqual(expected, parser.linesPerDir) {
		t.Fatalf("expected parsed lines:\n%#v\ngot:\n%#v\n", expected, parser.linesPerDir)
	}

}

func getTempDir(t *testing.T, prefix string) string {
	tmp, err := ioutil.TempDir("", prefix)
	if err != nil {
		t.Fatalf("failed to create a temporary directory: %v", err)
	}
	return tmp
}

type testDir struct {
	dirs     map[string]*testDir
	symlinks map[string]string
	files    map[string]string
}

func getDirsForMultipleTest() map[string]*testDir {
	dirs := make(map[string]*testDir, 3)
	for _, name := range []string{"etc", "lib", "user"} {
		upperName := strings.ToUpper(name)
		aaaDir := &testDir{
			files: map[string]string{
				"cfg.test": getValidContents("a", "1", fmt.Sprintf("%s AAA A", upperName)),
			},
		}
		bbbDir := &testDir{
			files: map[string]string{
				"cfg.test": getValidContents("b", "2", fmt.Sprintf("%s BBB B", upperName)),
			},
		}
		commonDir := &testDir{
			files: map[string]string{
				"a.test": getValidContents("a", "1", fmt.Sprintf("%s COMMON A", upperName)),
				"b.test": getValidContents("b", "2", fmt.Sprintf("%s COMMON B", upperName)),
			},
		}
		cfgDir := &testDir{
			dirs: map[string]*testDir{
				"aaa":    aaaDir,
				"bbb":    bbbDir,
				"common": commonDir,
			},
		}
		namedDir := &testDir{
			dirs: map[string]*testDir{
				"cfg": cfgDir,
			},
		}
		dirs[name] = namedDir
	}
	return dirs
}

func newTestDir() *testDir {
	return &testDir{
		dirs:     make(map[string]*testDir),
		symlinks: make(map[string]string),
		files:    make(map[string]string),
	}
}

func dirsWithTestDir(layout *testDir, dirs ...string) *testDir {
	if len(dirs) < 1 {
		return layout
	}
	top := newTestDir()
	bottom := top
	for _, d := range dirs[:len(dirs)-1] {
		td := newTestDir()
		bottom.dirs[d] = td
		bottom = td
	}
	lastDir := dirs[len(dirs)-1]
	bottom.dirs[lastDir] = layout
	return top
}

func createLayout(dir string, layout *testDir) error {
	if err := os.Mkdir(dir, 0755); err != nil {
		return fmt.Errorf("could not create directory %q: %v", dir, err)
	}
	for name, subLayout := range layout.dirs {
		full := filepath.Join(dir, name)
		if err := createLayout(full, subLayout); err != nil {
			return err
		}
	}
	for name, contents := range layout.files {
		full := filepath.Join(dir, name)
		if err := ioutil.WriteFile(full, []byte(contents), 0644); err != nil {
			return fmt.Errorf("could not write a file %q: %v", full, err)
		}
	}
	for target, name := range layout.symlinks {
		full := filepath.Join(dir, name)
		if err := os.Symlink(target, full); err != nil {
			return fmt.Errorf("could not create a symlink %q pointing to %q", full, target)
		}
	}
	return nil
}

func getValidContents(kind, version string, lines ...string) string {
	b := bytes.Buffer{}
	b.WriteString(fmt.Sprintf("%s\n%s\n", kind, version))
	for _, l := range lines {
		b.WriteString(fmt.Sprintf("%s\n", l))
	}
	return b.String()
}

func getTestParserSetup(kind, version string) *ParserSetup {
	return getParserSetup(kind, version, &testNoopParser{})
}

func getParserSetup(kind, version string, parser Parser) *ParserSetup {
	return &ParserSetup{
		Kind:    kind,
		Version: version,
		Parser:  parser,
	}
}

func getSubdirSetup(dir string, kinds ...string) *SubdirSetup {
	return &SubdirSetup{
		Subdir: dir,
		Kinds:  kinds,
	}
}
