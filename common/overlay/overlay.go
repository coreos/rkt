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

package overlay

import (
	"fmt"
	"syscall"

	"github.com/coreos/rkt/pkg/label"
	"github.com/hashicorp/errwrap"
)

// MountCfg contains the needed data to construct the overlay mount syscall.
// The Lower and Upper fields are paths to the filesystems to be merged. The
// Work field should be an empty directory. Dest is where the mount will be
// located. Lbl is an SELinux label.
type MountCfg struct {
	Lower,
	Upper,
	Work,
	Dest,
	Lbl string
}

// Mount mounts the upper and lower directories to the destination directory.
// The MountCfg struct supplies information required to build the mount system
// call.
func Mount(cfg *MountCfg) error {
	opts := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", cfg.Lower, cfg.Upper, cfg.Work)
	opts = label.FormatMountLabel(opts, cfg.Lbl)
	if err := syscall.Mount("overlay", cfg.Dest, "overlay", 0, opts); err != nil {
		return errwrap.Wrap(fmt.Errorf("error mounting overlay with options '%s' and dest '%s'", opts, cfg.Dest), err)
	}

	return nil
}
