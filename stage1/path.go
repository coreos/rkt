package main

import (
	"path/filepath"

	"github.com/coreos/rocket/app-container/schema/types"
	"github.com/coreos/rocket/path"
)

const (
	servicesDir = path.Stage1Dir + "/usr/lib/systemd/system"
	wantsDir    = servicesDir + "/default.target.wants"
)

// ServiceName returns a sanitized (escaped) systemd service name
// for the given imageID
func ServiceName(imageID types.Hash) string {
	return imageID.String() + ".service"
}

// WantsPath returns the systemd "wants" directory in root
func WantsPath(root string) string {
	return filepath.Join(root, wantsDir)
}

// ServicesPath returns the systemd "services" directory in root
func ServicesPath(root string) string {
	return filepath.Join(root, servicesDir)
}

// ServiceFilePath returns the path to the systemd service file
// path for the given imageID
func ServiceFilePath(root string, imageID types.Hash) string {
	return filepath.Join(root, servicesDir, ServiceName(imageID))
}

// WantLinkPath returns the systemd "want" symlink path for the
// given imageID
func WantLinkPath(root string, imageID types.Hash) string {
	return filepath.Join(root, wantsDir, ServiceName(imageID))
}

// WantUnitLinkPath returns the systemd "want" symlink path for the
// given unit
func WantUnitLinkPath(root string, unit string) string {
	return filepath.Join(root, wantsDir, unit)
}
