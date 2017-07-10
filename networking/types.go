// Copyright 2017 The rkt Authors
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
// limitations under the License.package networking

package networking

import (
	cnitypes "github.com/containernetworking/cni/pkg/types"

	"github.com/containernetworking/cni/pkg/ns"
	"github.com/rkt/rkt/networking/netinfo"
	stage1commontypes "github.com/rkt/rkt/stage1/common/types"
)

// Networking describes the networking details of a pod.
type Networking struct {
	Pod            *stage1commontypes.Pod
	LocalConfigDir string
	PodNS          ns.NetNS
	nets           []ActiveNet

	// Custom networking providers (like VMs) might want to override how
	// a network is actually created

	CreateNS bool // Whether or not the networking framework should create a
	// network namespace
	AddNet NetFn
	DelNet NetFn
}

// ActiveNet is a CNI configuration and, after execution, its result
type ActiveNet struct {
	ConfBytes []byte
	Conf      *cnitypes.NetConf
	Runtime   *netinfo.NetInfo
}

type NetFn func(*Networking, *ActiveNet) error
