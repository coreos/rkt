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

//+build linux

package main

import (
	"path/filepath"
	"strings"

	"github.com/coreos/rocket/Godeps/_workspace/src/github.com/appc/spec/schema"
	"github.com/coreos/rocket/common"
)

const (
	envDir          = "/rkt/env" // TODO(vc): perhaps this doesn't belong in /rkt?
	unitsDir        = "/usr/lib/systemd/system"
	defaultWantsDir = unitsDir + "/default.target.wants"
	socketsWantsDir = unitsDir + "/sockets.target.wants"
)

// ServiceUnitName returns a systemd service unit name for the given app
func ServiceUnitName(app *schema.RuntimeApp) string {
	return app.Name.EscapedString() + ".service"
}

// ServiceUnitPath returns the path to the systemd service file for the given
// app
func ServiceUnitPath(root string, app *schema.RuntimeApp) string {
	return filepath.Join(common.Stage1RootfsPath(root), unitsDir, ServiceUnitName(app))
}

// RelEnvFilePath returns the path to the environment file for the given app
// relative to the container's root
func RelEnvFilePath(app *schema.RuntimeApp) string {
	return filepath.Join(envDir, app.Name.EscapedString())
}

// EnvFilePath returns the path to the environment file for the given app
func EnvFilePath(root string, app *schema.RuntimeApp) string {
	return filepath.Join(common.Stage1RootfsPath(root), RelEnvFilePath(app))
}

// ServiceWantPath returns the systemd default.target want symlink path for the
// given app
func ServiceWantPath(root string, app *schema.RuntimeApp) string {
	return filepath.Join(common.Stage1RootfsPath(root), defaultWantsDir, ServiceUnitName(app))
}

// InstantiatedPrepareAppUnitName returns the systemd service unit name for prepare-app
// instantiated for the given root
func InstantiatedPrepareAppUnitName(app *schema.RuntimeApp) string {
	// Naming respecting escaping rules, see systemd.unit(5) and systemd-escape(1)
	escaped_root := common.RelAppRootfsPath(&app.Name)
	escaped_root = strings.Replace(escaped_root, "-", "\\x2d", -1)
	escaped_root = strings.Replace(escaped_root, "/", "-", -1)
	return "prepare-app@" + escaped_root + ".service"
}

// SocketUnitName returns a systemd socket unit name for the given app
func SocketUnitName(app *schema.RuntimeApp) string {
	return app.Name.EscapedString() + ".socket"
}

// SocketUnitPath returns the path to the systemd socket file for the given app
func SocketUnitPath(root string, app *schema.RuntimeApp) string {
	return filepath.Join(common.Stage1RootfsPath(root), unitsDir, SocketUnitName(app))
}

// SocketWantPath returns the systemd sockets.target.wants symlink path for the
// given app
func SocketWantPath(root string, app *schema.RuntimeApp) string {
	return filepath.Join(common.Stage1RootfsPath(root), socketsWantsDir, SocketUnitName(app))
}
