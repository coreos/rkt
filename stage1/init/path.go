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

//+build linux

package main

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/coreos/rkt/common"
)

const (
	envDir          = "/rkt/env" // TODO(vc): perhaps this doesn't belong in /rkt?
	unitsDir        = "/usr/lib/systemd/system"
	defaultWantsDir = unitsDir + "/default.target.wants"
	socketsWantsDir = unitsDir + "/sockets.target.wants"
)

// ServiceUnitName returns a systemd service unit name based on the position of the
// app in the pod manifest.
func ServiceUnitName(index int) string {
	return fmt.Sprintf("app-%d.service", index)
}

// ServiceUnitPath returns the path to the systemd service file based on the position
// of the app in the pod manifest.
func ServiceUnitPath(root string, index int) string {
	return filepath.Join(common.Stage1RootfsPath(root), unitsDir, ServiceUnitName(index))
}

// RelEnvFilePath returns the path to the environment file relative to the pod's root.
func RelEnvFilePath(index int) string {
	return filepath.Join(envDir, strconv.Itoa(index))
}

// EnvFilePath returns the path to the environment file based on the position of the
// app in the pod manifest.
func EnvFilePath(root string, index int) string {
	return filepath.Join(common.Stage1RootfsPath(root), RelEnvFilePath(index))
}

// ServiceWantPath returns the systemd default.target want symlink path based on the
// position of the app in the pod manifest.
func ServiceWantPath(root string, index int) string {
	return filepath.Join(common.Stage1RootfsPath(root), defaultWantsDir, ServiceUnitName(index))
}

// InstantiatedPrepareAppUnitName returns the systemd service unit name for prepare-app
// instantiated for the given root based on its position in the pod manifest.
func InstantiatedPrepareAppUnitName(index int) string {
	// Naming respecting escaping rules, see systemd.unit(5) and systemd-escape(1)
	escaped_root := common.RelAppRootfsPath(index)
	escaped_root = strings.Replace(escaped_root, "-", "\\x2d", -1)
	escaped_root = strings.Replace(escaped_root, "/", "-", -1)
	return "prepare-app@" + escaped_root + ".service"
}

// SocketUnitName returns a systemd socket unit name based on the position of the app in
// the pod manifest.
func SocketUnitName(index int) string {
	return fmt.Sprintf("app-%d.socket", index)
}

// SocketUnitPath returns the path to the systemd socket file based on the position of
// the app in the pod manifest.
func SocketUnitPath(root string, index int) string {
	return filepath.Join(common.Stage1RootfsPath(root), unitsDir, SocketUnitName(index))
}

// SocketWantPath returns the systemd sockets.target.wants symlink path based on the position
// of the app in the pod manifest.
func SocketWantPath(root string, index int) string {
	return filepath.Join(common.Stage1RootfsPath(root), socketsWantsDir, SocketUnitName(index))
}
