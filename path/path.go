package path

//
// Functions defining paths that are shared by different parts of rkt
// (e.g. stage0 and stage1)
//

import (
	"path/filepath"

	"github.com/coreos/rocket/app-container/schema/types"
)

const (
	Stage1Dir = "/stage1"
	stage2Dir = "/opt/stage2"
)

// Stage1RootfsPath returns the directory in root containing the rootfs for stage1
func Stage1RootfsPath(root string) string {
	return filepath.Join(root, Stage1Dir)
}

// ContainerManifestPath returns the path in root to the Container Runtime Manifest
func ContainerManifestPath(root string) string {
	return filepath.Join(root, "container")
}

// AppImagePath returns the path where an app image (i.e. unpacked ACI) is rooted (i.e.
// where its contents are extracted during stage0), based on the app image ID.
func AppImagePath(root string, imageID types.Hash) string {
	return filepath.Join(root, Stage1Dir, stage2Dir, imageID.String())
}

// AppRootfsPath returns the path to an app's rootfs.
// imageID should be the app image ID.
func AppRootfsPath(root string, imageID types.Hash) string {
	return filepath.Join(AppImagePath(root, imageID), "rootfs")
}

// RelAppImagePath returns the path of an application image relative to the
// stage1 chroot
func RelAppImagePath(imageID types.Hash) string {
	return filepath.Join(stage2Dir, imageID.String())
}

// RelAppImagePath returns the path of an application's rootfs relative to the
// stage1 chroot
func RelAppRootfsPath(imageID types.Hash) string {
	return filepath.Join(RelAppImagePath(imageID), "rootfs")
}

// ImageManifestPath returns the path to the app's manifest file inside the expanded ACI.
// id should be the app image ID.
func ImageManifestPath(root string, imageID types.Hash) string {
	return filepath.Join(AppImagePath(root, imageID), "app")
}
