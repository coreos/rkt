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

//+build linux

package common

import (
	"errors"
	"fmt"
	"strings"

	stage1commontypes "github.com/coreos/rkt/stage1/common/types"

	"github.com/appc/spec/schema/types"
	"github.com/coreos/go-systemd/unit"
)

var (
	ErrTooManySeccompIsolators = errors.New("too many seccomp isolators specified")
)

// Systemd filter mode, see
// https://www.freedesktop.org/software/systemd/man/systemd.exec.html#SystemCallFilter=
const (
	sdBlacklistMode = "~"
	sdWhitelistMode = ""
)

// getSeccompFilter gets an appc seccomp set and an optional error mode,
// returning those values in a format suitable for systemd consumption.
func getSeccompFilter(opts []*unit.UnitOption, p *stage1commontypes.Pod, unprivileged bool, isolators types.Isolators) ([]*unit.UnitOption, bool, error) {
	var filterMode string
	flag := ""
	seccompIsolators := 0
	seccompErrno := ""
	noNewPrivs := false
	var err error
	var seccompSet []string
	for _, i := range isolators {
		if seccomp, ok := i.Value().(types.LinuxSeccompSet); ok {
			seccompIsolators++
			// By appc spec, only one seccomp isolator per app is allowed
			if seccompIsolators > 1 {
				return nil, false, ErrTooManySeccompIsolators
			}
			switch i.Name {
			case types.LinuxSeccompRemoveSetName:
				filterMode = sdBlacklistMode
				seccompSet, flag, err = parseLinuxSeccompSet(p, seccomp)
				if err != nil {
					return nil, false, err
				}
				if flag == "empty" {
					// Opt-in to rkt default whitelist
					seccompSet = nil
					break
				}
			case types.LinuxSeccompRetainSetName:
				filterMode = sdWhitelistMode
				seccompSet, flag, err = parseLinuxSeccompSet(p, seccomp)
				if err != nil {
					return nil, false, err
				}
				if flag == "all" {
					// Opt-out seccomp filtering
					return opts, false, nil
				}
			}
			seccompErrno = string(seccomp.Errno())
		}
	}

	// If unset, use rkt default whitelist
	if len(seccompSet) == 0 {
		filterMode = sdWhitelistMode
		seccompSet = RktDefaultSeccompWhitelist
	}

	// Append computed options
	if seccompErrno != "" {
		opts = append(opts, unit.NewUnitOption("Service", "SystemCallErrorNumber", seccompErrno))
	}
	// SystemCallFilter options are written down in batches of 50 entries per line, because
	// filtering sets may be quite large and overlong lines break unit serialization.
	seccompStr := ""
	for i, s := range seccompSet {
		next := i + 1
		seccompStr += (s + " ")
		if next%50 == 0 || next == len(seccompSet) {
			opts = append(opts, unit.NewUnitOption("Service", "SystemCallFilter", fmt.Sprintf("%s%s", filterMode, seccompStr)))
			seccompStr = ""
		}
	}
	// In order to install seccomp filters, unprivileged process must first set no-news-privs.
	if unprivileged {
		noNewPrivs = true
	}

	return opts, noNewPrivs, nil
}

// parseLinuxSeccompSet gets an appc LinuxSeccompSet and returns an array
// of values suitable for systemd SystemCallFilter.
func parseLinuxSeccompSet(p *stage1commontypes.Pod, s types.LinuxSeccompSet) (syscallFilter []string, flag string, err error) {
	for _, item := range s.Set() {
		if item[0] == '@' {
			// Wildcards
			wildcard := strings.SplitN(string(item), "/", 2)
			if len(wildcard) != 2 {
				continue
			}
			scope := wildcard[0]
			name := wildcard[1]
			switch scope {
			case "@appc.io":
				// appc-reserved wildcards
				switch name {
				case "all":
					return nil, "all", nil
				case "empty":
					return nil, "empty", nil
				}
			case "@docker":
				// Docker-originated wildcards
				switch name {
				case "default-blacklist":
					syscallFilter = append(syscallFilter, DockerDefaultSeccompBlacklist...)
				case "default-whitelist":
					syscallFilter = append(syscallFilter, DockerDefaultSeccompWhitelist...)
				}
			case "@rkt":
				// Custom rkt wildcards
				switch name {
				case "default-blacklist":
					syscallFilter = append(syscallFilter, RktDefaultSeccompBlacklist...)
				case "default-whitelist":
					syscallFilter = append(syscallFilter, RktDefaultSeccompWhitelist...)
				}
			case "@systemd":
				// Custom systemd wildcards (systemd >= 231)
				_, systemdVersion, err := GetFlavor(p)
				if err != nil || systemdVersion < 231 {
					return nil, "", errors.New("Unsupported or unknown systemd version, seccomp groups need systemd >= v231")
				}
				switch name {
				case "clock":
					syscallFilter = append(syscallFilter, "@clock")
				case "default-whitelist":
					syscallFilter = append(syscallFilter, "@default")
				case "mount":
					syscallFilter = append(syscallFilter, "@mount")
				case "network-io":
					syscallFilter = append(syscallFilter, "@network-io")
				case "obsolete":
					syscallFilter = append(syscallFilter, "@obsolete")
				case "privileged":
					syscallFilter = append(syscallFilter, "@privileged")
				case "process":
					syscallFilter = append(syscallFilter, "@process")
				case "raw-io":
					syscallFilter = append(syscallFilter, "@raw-io")
				}
			}
		} else {
			// Plain syscall name
			syscallFilter = append(syscallFilter, string(item))
		}
	}
	return syscallFilter, "", nil
}
