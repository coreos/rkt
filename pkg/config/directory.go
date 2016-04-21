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

// This package provides a simple configuration walker. It imposes a
// configuration structure as described in following paragraphs.
//
// Usually a project reads configuration from several places, like a
// vendor configuration in /usr/lib/project, an admin configuration in
// /etc/project and a user configuration in ~/.config/projects. These
// paths are called toplevel directories in this package.
//
// A project can have several modules. Each of them can have a
// separate directory containing module-specific documentation. For
// example a project could have two modules, "frontend" and "backend"
// and their configuration files could be stored in, respectively,
// "frontend" and "backend" subdirectories of toplevel directories.
// With the above examples of the toplevel directories, it means that
// each module gets three directories - "frontend" module gets
// /usr/lib/project/frontend, /etc/project/frontend and
// ~/.config/project/frontend, and "backend" module gets similar, but
// with a "backend" subdirectory instead of "frontend". The toplevel
// directory with a module subdirectory is called configuration
// directory and its basename is called configuration directory
// basename (often shortened to cdb).
//
// Each module can have different kinds and versions of configuration
// files stored in its configuration directories. For example, a
// frontend could have a separate directory for configuring
// authentication (of a general kind and a kind specific to some
// service), a separate one for configuring paths and a separate one
// for configuring some important piece of frontend. Backend could
// have a separate directory for specifying some low-level mechanisms
// and a separate directory for some generator options. In the end,
// the directory layout would like like follows:
//
//  <toplevel directory> (/usr/lib/project, etc...)
//  + frontend
//  | + auth
//  | + paths
//  | + piece
//  + backend
//    + low-level
//    + generator
//
// The "auth", "paths", "piece", "low-level" and "generator"
// directories are called simply subdirectories in this package.
//
// Frontend module can have its configuration in an INI format in
// files with a "ini" extension, and backend configuration, in, for
// example, a YAML format in files with a "yml" extension.
//
// Each subdirectory can contain other files and directories. The
// directories and the files with a different extension than "ini" or
// "yml" are ignored by, respectively, frontend and backend.
//
// The above translates to code as follows:
//
// iniType := &someIniConfigType{} // implements config.Type
// ymlType := &someYAMLConfigType{} // implements config.Type
// feDir := config.NewDirectory("frontend", iniType)
// beDir := config.NewDirectory("backend", ymlType)
// feSubdirs := []config.SubdirSetup{
//     {
//         Subdir: "auth",
//         Kind:   []string{"generic-auth", "service-specific-auth"},
//     },
//     // and similar for "paths" and "piece" subdirs
// }
// feParsers := []config.ParserSetup{
//     {
//         Kind: "generic-auth",
//         Version: "1",
//         Parser: &genericAuthParser() // implements config.Parser
//     },
//     {
//         Kind: "service-specific-auth",
//         Version: "1",
//         Parser: &specificAuthParser() // implements config.Parser
//     },
//     {
//         Kind: "service-specific-auth",
//         Version: "2", // new, incompatible version
//         Parser: &specificAuth2Parser() // implements config.Parser
//     },
//     // and similar for the kinds in "paths" and "piece" subdirs
// }
// beSubdirs := []config.SubdirSetup{
//     // similar things like in feSubdirs
// }
// beParsers := []config.ParserSetup{
//     // similar things like in feParsers
// }
// if err := feDir.RegisterSubdirectories(feSubdirs); err != nil {
//     ...
// }
// if err := feDir.RegisterParsers(feParsers); err != nil {
//     ...
// }
// // similar to do with beDir and beSubdirs and beParsers
// toplevelDirs := []string{"/usr/lib/project", "/etc/project", getUserConfigPath()}
// if err := feDir.WalkDirectories(toplevelDirs); err != nil {
//     ...
// }
// if err := beDir.WalkDirectories(toplevelDirs); err != nil {
//     ...
// }
package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hashicorp/errwrap"
)

// Parser is an interface that wraps the Parse method.
type Parser interface {
	// Parse takes a configuration directory path index and raw
	// contents of a file from a subdirectory of the configuration
	// directory. Storing the results of the parsing is a
	// responsibility of the Parser interface implementation. The
	// parser can do whatever it wants with the given path index.
	//
	// If any error happens during parsing, it should be returned.
	Parse(idx *PathIndex, raw []byte) error
}

// Type describes the files Directory should be able to parse with its
// registered parsers.
type Type interface {
	// Extension returns an extension of a file to tell Directory
	// which files are eligible for parsing. The returned string
	// should not contain the leading dot, e.g. "json".
	Extension() string

	// GetKindAndVersion should try to initially parse the
	// contents of the file to get its kind and version. It should
	// return an error if that fails because of invalid file
	// format or missing kind or flavor fields.
	GetKindAndVersion(raw []byte) (string, string, error)
}

// ParserSetup is a convenience struct for registering many parsers
// with RegisterParsers, so there is only a single point of
// failure. For a meaning of the fields of ParserSetup, see the
// RegisterParser function.
type ParserSetup struct {
	Kind    string
	Version string
	Parser  Parser
}

// SubdirSetup is a convenience struct for registering many
// subdirectories with RegisterSubdirectories, so there is only a
// single point of failure. For a meaning of the fields of
// SubdirSetup, see the RegisterSubdirectory function.
type SubdirSetup struct {
	Subdir string
	Kinds  []string
}

// PathIndex wraps the configuration directory and its index in the
// list of passed toplevel paths to WalkDirectory. It also provides
// additional data like subdirectory and the filename of a parsed
// file. Note that Index is tied only with Path, so Parser can get
// different instances of PathIndex with the same Index and Path
// fields, but the different Subdirectory and Filename fields.
type PathIndex struct {
	Index        int
	Path         string
	Subdirectory string
	Filename     string
}

// Directory wraps a configuration directory. A configuration
// directory can have several subdirectories, registered with
// RegisterSubdirectory or with the convenience RegisterSubdirectories
// function. Each subdirectory can contain configuration files of
// several kinds. The directory also has configuration parsers
// associated with it, so it knows how to parse a specific kind of a
// configuration file. The parsers are registered with RegisterParser
// or with a convenience RegisterParsers function. The directory
// understands only a single configuration type described by Type.
type Directory struct {
	directory      string
	configType     Type
	configSubdirs  map[string][]string
	parsersForKind map[string]map[string]Parser
}

func (pi *PathIndex) FilePath() string {
	return filepath.Join(pi.Path, pi.Subdirectory, pi.Filename)
}

// NewDirectory gets an object wrapping a configuration directory of a
// specific typ in a toplevel directory. The cdbn parameter is a
// basename of a configuration directory (so it is just "frontend"
// instead of "/etc/project/frontend").
func NewDirectory(cdbn string, configType Type) *Directory {
	return &Directory{
		directory:      cdbn,
		configType:     configType,
		configSubdirs:  make(map[string][]string),
		parsersForKind: make(map[string]map[string]Parser),
	}
}

// RegisterParsers registers given parsers. It returns an error on the
// first registration failure. See RegisterParser for details.
func (d *Directory) RegisterParsers(setups []*ParserSetup) error {
	for _, setup := range setups {
		if err := d.RegisterParser(setup.Kind, setup.Version, setup.Parser); err != nil {
			return err
		}
	}
	return nil
}

// RegisterParser registers a parser. The registered parser will be
// used to parse configuration files of a given kind and a
// version. This function will return an error if there is already a
// registered parser for the kind and the version.
func (d *Directory) RegisterParser(kind, version string, parser Parser) error {
	if len(kind) == 0 {
		return fmt.Errorf("empty kind string for version %q when registering a config parser", version)
	}
	if len(version) == 0 {
		return fmt.Errorf("empty version string for kind %q when registering a config parser", kind)
	}
	if parser == nil {
		return fmt.Errorf("trying to register a nil parser for kind %q and version %q", kind, version)
	}
	if _, err := d.getParser(kind, version); err == nil {
		return fmt.Errorf("parser for kind %q and version %q already exists", kind, version)
	}
	if _, ok := d.parsersForKind[kind]; !ok {
		d.parsersForKind[kind] = make(map[string]Parser)
	}
	d.parsersForKind[kind][version] = parser
	return nil
}

// RegisterSubdirectories registers given subdirectories. It returns
// an error on the first registration failure. See
// RegisterSubdirectory for details.
func (d *Directory) RegisterSubdirectories(setups []*SubdirSetup) error {
	for _, setup := range setups {
		if err := d.RegisterSubdirectory(setup.Subdir, setup.Kinds); err != nil {
			return err
		}
	}
	return nil
}

// RegisterSubdirectory registers a subdirectory. The registered
// subdirectory should hold configuration files only of the given
// kinds. Subsequent calls to the RegisterSubdirectory function with
// the already registered directory will merge the registered kinds
// with the new ones.
func (d *Directory) RegisterSubdirectory(dir string, kinds []string) error {
	if len(dir) == 0 {
		return fmt.Errorf("trying to register empty config subdirectory for kinds %v", kinds)
	}
	if len(kinds) == 0 {
		return fmt.Errorf("kinds array cannot be empty when registering config subdirectory %q", dir)
	}
	allKinds := toArray(toSet(append(d.configSubdirs[dir], kinds...)))
	sort.Strings(allKinds)
	d.configSubdirs[dir] = allKinds
	return nil
}

// WalkDirectories walks the directory config in the given toplevel
// directories and parses configuration files.
func (d *Directory) WalkDirectories(dirs ...string) error {
	for idx, topDir := range dirs {
		configDir := filepath.Join(topDir, d.directory)
		idx := &PathIndex{
			Path:  configDir,
			Index: idx,
		}
		if err := d.walkConfigDirectory(idx); err != nil {
			return err
		}
	}
	return nil
}

func (d *Directory) walkConfigDirectory(idx *PathIndex) error {
	if valid, err := validDir(idx.Path); err != nil {
		return err
	} else if !valid {
		return nil
	}
	return d.readConfigDir(idx)
}

func (d *Directory) readConfigDir(idx *PathIndex) error {
	for csd, kinds := range d.configSubdirs {
		subdir := filepath.Join(idx.Path, csd)
		if valid, err := validDir(subdir); err != nil {
			return err
		} else if !valid {
			continue
		}
		configWalker := d.getConfigWalker(idx, kinds, subdir)
		if err := filepath.Walk(subdir, configWalker); err != nil {
			return err
		}
	}
	return nil
}

type subdirectory struct {
	parent *Directory
	idx    *PathIndex
	kinds  []string
}

func (d *Directory) getConfigWalker(idx *PathIndex, kinds []string, root string) filepath.WalkFunc {
	sd := &subdirectory{
		parent: d,
		idx:    idx.copyWithSubdir(root),
		kinds:  kinds,
	}
	return func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		return sd.readFile(info, path)
	}
}

func (idx *PathIndex) copyWithSubdir(path string) *PathIndex {
	return &PathIndex{
		Index:        idx.Index,
		Path:         idx.Path,
		Subdirectory: filepath.Base(path),
	}
}

func (sd *subdirectory) readFile(info os.FileInfo, path string) error {
	if valid, err := sd.parent.validConfigFile(info); err != nil {
		return err
	} else if !valid {
		return nil
	}
	if err := sd.parseConfigFile(path); err != nil {
		return err
	}
	return nil
}

func (d *Directory) validConfigFile(info os.FileInfo) (bool, error) {
	// TODO: support symlinks?
	mode := info.Mode()
	switch {
	case mode.IsDir():
		return false, filepath.SkipDir
	case mode.IsRegular():
		allowedExtension := fmt.Sprintf(".%s", d.configType.Extension())
		if filepath.Ext(info.Name()) == allowedExtension {
			return true, nil
		}
		return false, nil
	default:
		return false, nil
	}
}

func (sd *subdirectory) parseConfigFile(path string) error {
	raw, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	kind, version, err := sd.parent.configType.GetKindAndVersion(raw)
	if err != nil {
		return errwrap.Wrap(fmt.Errorf("failed to get configuration kind and version from %q", path), err)
	}
	kindOk := false
	for _, allowedKind := range sd.kinds {
		if kind == allowedKind {
			kindOk = true
			break
		}
	}
	if !kindOk {
		dir := filepath.Dir(path)
		base := filepath.Base(path)
		kindsStr := strings.Join(wrap(sd.kinds, `"`, `"`), ", ")
		return fmt.Errorf("the configuration directory %q expects to have configuration files of kinds %s, but %q has kind of %q", dir, kindsStr, base, kind)
	}
	parser, err := sd.parent.getParser(kind, version)
	if err != nil {
		return err
	}
	if err := parser.Parse(sd.idx.copyWithFilename(path), raw); err != nil {
		return errwrap.Wrap(fmt.Errorf("failed to parse %q", path), err)
	}
	return nil
}

func (idx *PathIndex) copyWithFilename(path string) *PathIndex {
	return &PathIndex{
		Index:        idx.Index,
		Path:         idx.Path,
		Subdirectory: idx.Subdirectory,
		Filename:     filepath.Base(path),
	}
}

func (d *Directory) getParser(kind, version string) (Parser, error) {
	parsers, ok := d.parsersForKind[kind]
	if !ok {
		return nil, fmt.Errorf("no parser available for configuration of kind %q", kind)
	}
	parser, ok := parsers[version]
	if !ok {
		return nil, fmt.Errorf("no parser available for configuration of kind %q and version %q", kind, version)
	}
	return parser, nil
}

// miscellaneous functions

func wrap(strs []string, prefix, suffix string) []string {
	wrapped := make([]string, 0, len(strs))
	for _, str := range strs {
		wrapped = append(wrapped, fmt.Sprintf("%s%s%s", prefix, str, suffix))
	}
	return wrapped
}

func toSet(a []string) map[string]struct{} {
	s := make(map[string]struct{})
	for _, v := range a {
		s[v] = struct{}{}
	}
	return s
}

func toArray(s map[string]struct{}) []string {
	a := make([]string, 0, len(s))
	for k := range s {
		a = append(a, k)
	}
	return a
}

func validDir(path string) (bool, error) {
	fi, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if !fi.IsDir() {
		return false, fmt.Errorf("expected %q to be a directory", path)
	}
	return true, nil
}
