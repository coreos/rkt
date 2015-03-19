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

// Package common defines values shared by different parts
// of rkt (e.g. stage0 and stage1)
package common

import (
	"fmt"
	"path/filepath"

	"github.com/coreos/rocket/Godeps/_workspace/src/github.com/appc/spec/aci"
	"github.com/coreos/rocket/Godeps/_workspace/src/github.com/appc/spec/schema/types"
)

const (
	stage1Dir = "/stage1"
	stage2Dir = "/opt/stage2"

	MetadataServiceIP      = "169.254.169.255"
	MetadataServicePubPort = 80
	MetadataServicePrvPort = 2375
)

// Stage1ImagePath returns the path where the stage1 app image (unpacked ACI) is rooted,
// (i.e. where its contents are extracted during stage0).
func Stage1ImagePath(root string) string {
	return filepath.Join(root, stage1Dir)
}

// Stage1RootfsPath returns the path to the stage1 rootfs
func Stage1RootfsPath(root string) string {
	return filepath.Join(Stage1ImagePath(root), aci.RootfsDir)
}

// Stage1ManifestPath returns the path to the stage1's manifest file inside the expanded ACI.
func Stage1ManifestPath(root string) string {
	return filepath.Join(Stage1ImagePath(root), aci.ManifestFile)
}

// ContainerManifestPath returns the path in root to the Container Runtime Manifest
func ContainerManifestPath(root string) string {
	return filepath.Join(root, "container")
}

// AppImagesPath returns the path where the app images live
func AppImagesPath(root string) string {
	return filepath.Join(Stage1RootfsPath(root), stage2Dir)
}

// AppImagePath returns the path where an app image (i.e. unpacked ACI) is rooted (i.e.
// where its contents are extracted during stage0), based on the app name.
func AppImagePath(root string, appName *types.ACName) string {
	return filepath.Join(AppImagesPath(root), appName.EscapedString())
}

// AppRootfsPath returns the path to an app's rootfs.
// name should be the unique app name.
func AppRootfsPath(root string, appName *types.ACName) string {
	return filepath.Join(AppImagePath(root, appName), aci.RootfsDir)
}

// RelAppImagePath returns the path of an application image relative to the
// stage1 chroot
func RelAppImagePath(appName *types.ACName) string {
	return filepath.Join(stage2Dir, appName.EscapedString())
}

// RelAppImagePath returns the path of an application's rootfs relative to the
// stage1 chroot
func RelAppRootfsPath(appName *types.ACName) string {
	return filepath.Join(RelAppImagePath(appName), aci.RootfsDir)
}

// ImageManifestPath returns the path to the app's manifest file inside the expanded ACI.
func ImageManifestPath(root string, appName *types.ACName) string {
	return filepath.Join(AppImagePath(root, appName), aci.ManifestFile)
}

// MetadataServicePrivateURL returns the private URL used to host the metadata service
func MetadataServicePrivateURL() string {
	return fmt.Sprintf("http://127.0.0.1:%v", MetadataServicePrvPort)
}

// MetadataServicePublicURL returns the public URL used to host the metadata service
func MetadataServicePublicURL() string {
	return fmt.Sprintf("http://%v:%v", MetadataServiceIP, MetadataServicePubPort)
}
