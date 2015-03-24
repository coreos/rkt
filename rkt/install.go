// Copyright 2015 CoreOS, Inc.
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
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	rocketGroup   = "rocket"
	groupFilePath = "/etc/group"
)

var (
	cmdInstall = &Command{
		Name:    "install",
		Summary: "Set up Rocket data directories with correct permissions",
		Usage:   "",
		Run:     runInstall,
		Flags:   &installFlags,
	}
	installFlags flag.FlagSet

	// dirs relative to globalFlags.Dir
	dirs = map[string]os.FileMode{
		".":          os.FileMode(0755),
		"cas":        os.FileMode(0775),
		"tmp":        os.FileMode(0775),
		"containers": os.FileMode(0700),
	}
)

type Group struct {
	Name  string
	Pass  string
	Gid   int
	Users []string
}

func init() {
	commands = append(commands, cmdInstall)
}

func parseGroupLine(line string, group *Group) {
	const (
		NameIdx = iota
		PassIdx
		GidIdx
		UsersIdx
	)

	if line == "" {
		return
	}

	splits := strings.Split(line, ":")
	if len(splits) < 4 {
		return
	}

	group.Name = splits[NameIdx]
	group.Pass = splits[PassIdx]
	group.Gid, _ = strconv.Atoi(splits[GidIdx])

	u := splits[UsersIdx]
	if u != "" {
		group.Users = strings.Split(u, ",")
	} else {
		group.Users = []string{}
	}
}

func parseGroupFile(path string) (group map[string]Group, err error) {
	groupFile, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer groupFile.Close()

	return parseGroups(groupFile)
}

func parseGroups(r io.Reader) (group map[string]Group, err error) {
	s := bufio.NewScanner(r)
	out := make(map[string]Group)

	for s.Scan() {
		if err := s.Err(); err != nil {
			return nil, err
		}

		text := s.Text()
		if text == "" {
			continue
		}

		p := Group{}
		parseGroupLine(text, &p)

		out[p.Name] = p
	}

	return out, nil
}

func lookupGid(groupName string) (gid int, err error) {
	groups, err := parseGroupFile(groupFilePath)
	if err != nil {
		return -1, fmt.Errorf("error parsing %q file: %v", groupFilePath, err)
	}

	group, ok := groups[groupName]
	if !ok {
		return -1, fmt.Errorf("%q group not found", groupName)
	}

	return group.Gid, nil
}

func setDirPermissions(path string, uid int, gid int, perm os.FileMode) error {
	if err := os.Chown(path, 0, gid); err != nil {
		return fmt.Errorf("error setting %q directory group: %v", path, err)
	}

	if err := os.Chmod(path, perm); err != nil {
		return fmt.Errorf("error setting %q directory permissions: %v", path, err)
	}

	return nil
}

func createDirStructure(gid int) error {
	for dir, perm := range dirs {
		path := filepath.Join(globalFlags.Dir, dir)

		if err := os.MkdirAll(path, perm); err != nil {
			return fmt.Errorf("error creating %q directory: %v", path, err)
		}

		if err := setDirPermissions(path, 0, gid, perm); err != nil {
			return err
		}
	}

	return nil
}

func runInstall(args []string) (exit int) {
	gid, err := lookupGid(rocketGroup)
	if err != nil {
		stderr("install: error looking up rocket gid: %v", err)
		return 1
	}

	err = createDirStructure(gid)
	if err != nil {
		stderr("install: error creating Rocket directory structure: %v", err)
		return 1
	}

	fmt.Println("Rocket directory structure successfully created.")

	return 0
}
