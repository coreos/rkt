// Copyright 2015 The rkt Authors
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

package dummy

// This is a dummy package to ensure that Godep vendors
// actool, which is used in building the stage1 ACI
import (
	_ "github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/aci"
	_ "github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/actool"
)

// Vendor in CNI plugins
import (
	_ "github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/cni/plugins/ipam/dhcp"
	_ "github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/cni/plugins/ipam/host-local"
	_ "github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/cni/plugins/main/bridge"
	_ "github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/cni/plugins/main/ipvlan"
	_ "github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/cni/plugins/main/macvlan"
	_ "github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/cni/plugins/main/ptp"
	_ "github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/cni/plugins/meta/flannel"
)

// Vendor in ACE
import (
	_ "github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/ace"
)
