//+build linux

package main

import (
	"path/filepath"

	"github.com/appc/spec/schema/types"
	"github.com/coreos/rocket/path"
)

const (
	unitsDir        = path.Stage1Dir + "/usr/lib/systemd/system"
	defaultWantsDir = unitsDir + "/default.target.wants"
	socketsWantsDir = unitsDir + "/sockets.target.wants"
)

// ServiceUnitName returns a systemd service unit name for the given imageID
func ServiceUnitName(imageID types.Hash) string {
	return imageID.String() + ".service"
}

// ServiceUnitPath returns the path to the systemd service file for the given
// imageID
func ServiceUnitPath(root string, imageID types.Hash) string {
	return filepath.Join(root, unitsDir, ServiceUnitName(imageID))
}

// ServiceWantPath returns the systemd default.target want symlink path for the
// given imageID
func ServiceWantPath(root string, imageID types.Hash) string {
	return filepath.Join(filepath.Join(root, defaultWantsDir), ServiceUnitName(imageID))
}

// SocketUnitName returns a systemd socket unit name for the given imageID
func SocketUnitName(imageID types.Hash) string {
	return imageID.String() + ".socket"
}

// SocketUnitPath returns the path to the systemd socket file for the given imageID
func SocketUnitPath(root string, imageID types.Hash) string {
	return filepath.Join(root, unitsDir, SocketUnitName(imageID))
}

// SocketWantPath returns the systemd sockets.target.wants symlink path for the
// given imageID
func SocketWantPath(root string, imageID types.Hash) string {
	return filepath.Join(filepath.Join(root, socketsWantsDir), SocketUnitName(imageID))
}
