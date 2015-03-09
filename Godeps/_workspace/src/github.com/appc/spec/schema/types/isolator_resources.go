package types

import (
	"encoding/json"
	"errors"

	"github.com/coreos/rocket/Godeps/_workspace/src/github.com/GoogleCloudPlatform/kubernetes/pkg/api/resource"
)

var (
	ErrDefaultTrue     = errors.New("default must be false")
	ErrDefaultRequired = errors.New("default must be true")
	ErrRequestNonEmpty = errors.New("request not supported by this resource, must be empty")
)

const (
	ResourceBlockBandwidthName   = "resource/block-bandwidth"
	ResourceBlockIOPSName        = "resource/block-iops"
	ResourceCPUName              = "resource/cpu"
	ResourceMemoryName           = "resource/memory"
	ResourceNetworkBandwidthName = "resource/network-bandwidth"
)

func init() {
	AddIsolatorValueConstructor(NewResourceBlockBandwidth)
	AddIsolatorValueConstructor(NewResourceBlockIOPS)
	AddIsolatorValueConstructor(NewResourceCPU)
	AddIsolatorValueConstructor(NewResourceMemory)
	AddIsolatorValueConstructor(NewResourceNetworkBandwidth)
}

func NewResourceBlockBandwidth() IsolatorValue {
	return &ResourceBlockBandwidth{}
}
func NewResourceBlockIOPS() IsolatorValue {
	return &ResourceBlockIOPS{}
}
func NewResourceCPU() IsolatorValue {
	return &ResourceCPU{}
}
func NewResourceNetworkBandwidth() IsolatorValue {
	return &ResourceNetworkBandwidth{}
}
func NewResourceMemory() IsolatorValue {
	return &ResourceMemory{}
}

type Resource interface {
	Limit() *resource.Quantity
	Request() *resource.Quantity
	Default() bool
}

type ResourceBase struct {
	val resourceValue
}

type resourceValue struct {
	Default bool               `json:"default"`
	Request *resource.Quantity `json:"request"`
	Limit   *resource.Quantity `json:"limit"`
}

func (r ResourceBase) Limit() *resource.Quantity {
	return r.val.Limit
}
func (r ResourceBase) Request() *resource.Quantity {
	return r.val.Request
}
func (r ResourceBase) Default() bool {
	return r.val.Default
}

func (r *ResourceBase) UnmarshalJSON(b []byte) error {
	return json.Unmarshal(b, &r.val)
}

func (r ResourceBase) AssertValid() error {
	return nil
}

type ResourceBlockBandwidth struct {
	ResourceBase
}

func (r ResourceBlockBandwidth) AssertValid() error {
	if r.Default() != true {
		return ErrDefaultRequired
	}
	if r.Request() != nil {
		return ErrRequestNonEmpty
	}
	return nil
}

func (r ResourceBlockBandwidth) Name() string {
	return ResourceBlockBandwidthName
}

type ResourceBlockIOPS struct {
	ResourceBase
}

func (r ResourceBlockIOPS) AssertValid() error {
	if r.Default() != true {
		return ErrDefaultRequired
	}
	if r.Request() != nil {
		return ErrRequestNonEmpty
	}
	return nil
}

func (r ResourceBlockIOPS) Name() string {
	return ResourceBlockIOPSName
}

type ResourceCPU struct {
	ResourceBase
}

func (r ResourceCPU) AssertValid() error {
	if r.Default() != false {
		return ErrDefaultTrue
	}
	return nil
}

func (r ResourceCPU) Name() string {
	return ResourceCPUName
}

type ResourceMemory struct {
	ResourceBase
}

func (r ResourceMemory) AssertValid() error {
	if r.Default() != false {
		return ErrDefaultTrue
	}
	return nil
}

func (r ResourceMemory) Name() string {
	return ResourceMemoryName
}

type ResourceNetworkBandwidth struct {
	ResourceBase
}

func (r ResourceNetworkBandwidth) AssertValid() error {
	if r.Default() != true {
		return ErrDefaultRequired
	}
	if r.Request() != nil {
		return ErrRequestNonEmpty
	}
	return nil
}

func (r ResourceNetworkBandwidth) Name() string {
	return ResourceNetworkBandwidthName
}
