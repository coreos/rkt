// Copyright 2015 The appc Authors
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

package types

import (
	"encoding/json"
	"errors"
	"unicode"
)

const (
	LinuxCapabilitiesRetainSetName = "os/linux/capabilities-retain-set"
	LinuxCapabilitiesRevokeSetName = "os/linux/capabilities-remove-set"
	LinuxNoNewPrivilegesName       = "os/linux/no-new-privileges"
	LinuxSeccompRemoveSetName      = "os/linux/seccomp-remove-set"
	LinuxSeccompRetainSetName      = "os/linux/seccomp-retain-set"
)

var LinuxIsolatorNames = make(map[ACIdentifier]struct{})

func init() {
	for name, con := range map[ACIdentifier]IsolatorValueConstructor{
		LinuxCapabilitiesRevokeSetName: func() IsolatorValue { return &LinuxCapabilitiesRevokeSet{} },
		LinuxCapabilitiesRetainSetName: func() IsolatorValue { return &LinuxCapabilitiesRetainSet{} },
		LinuxNoNewPrivilegesName:       func() IsolatorValue { v := LinuxNoNewPrivileges(false); return &v },
		LinuxSeccompRemoveSetName:      func() IsolatorValue { return &LinuxSeccompRemoveSet{} },
		LinuxSeccompRetainSetName:      func() IsolatorValue { return &LinuxSeccompRetainSet{} },
	} {
		AddIsolatorName(name, LinuxIsolatorNames)
		AddIsolatorValueConstructor(name, con)
	}
}

type LinuxNoNewPrivileges bool

func (l LinuxNoNewPrivileges) AssertValid() error {
	return nil
}

// TODO(lucab): both need to be clarified in spec,
// see https://github.com/appc/spec/issues/625
func (l LinuxNoNewPrivileges) multipleAllowed() bool {
	return true
}
func (l LinuxNoNewPrivileges) Conflicts() []ACIdentifier {
	return nil
}

func (l *LinuxNoNewPrivileges) UnmarshalJSON(b []byte) error {
	var v bool
	err := json.Unmarshal(b, &v)
	if err != nil {
		return err
	}

	*l = LinuxNoNewPrivileges(v)

	return nil
}

type AsIsolator interface {
	AsIsolator() (*Isolator, error)
}

type LinuxCapabilitiesSet interface {
	Set() []LinuxCapability
	AssertValid() error
}

type LinuxCapability string

type linuxCapabilitiesSetValue struct {
	Set []LinuxCapability `json:"set"`
}

type linuxCapabilitiesSetBase struct {
	val linuxCapabilitiesSetValue
}

func (l linuxCapabilitiesSetBase) AssertValid() error {
	if len(l.val.Set) == 0 {
		return errors.New("set must be non-empty")
	}
	return nil
}

// TODO(lucab): both need to be clarified in spec,
// see https://github.com/appc/spec/issues/625
func (l linuxCapabilitiesSetBase) multipleAllowed() bool {
	return true
}
func (l linuxCapabilitiesSetBase) Conflicts() []ACIdentifier {
	return nil
}

func (l *linuxCapabilitiesSetBase) UnmarshalJSON(b []byte) error {
	var v linuxCapabilitiesSetValue
	err := json.Unmarshal(b, &v)
	if err != nil {
		return err
	}

	l.val = v

	return err
}

func (l linuxCapabilitiesSetBase) Set() []LinuxCapability {
	return l.val.Set
}

type LinuxCapabilitiesRetainSet struct {
	linuxCapabilitiesSetBase
}

func NewLinuxCapabilitiesRetainSet(caps ...string) (*LinuxCapabilitiesRetainSet, error) {
	l := LinuxCapabilitiesRetainSet{
		linuxCapabilitiesSetBase{
			linuxCapabilitiesSetValue{
				make([]LinuxCapability, len(caps)),
			},
		},
	}
	for i, c := range caps {
		l.linuxCapabilitiesSetBase.val.Set[i] = LinuxCapability(c)
	}
	if err := l.AssertValid(); err != nil {
		return nil, err
	}
	return &l, nil
}

func (l LinuxCapabilitiesRetainSet) AsIsolator() (*Isolator, error) {
	b, err := json.Marshal(l.linuxCapabilitiesSetBase.val)
	if err != nil {
		return nil, err
	}
	rm := json.RawMessage(b)
	return &Isolator{
		Name:     LinuxCapabilitiesRetainSetName,
		ValueRaw: &rm,
		value:    &l,
	}, nil
}

type LinuxCapabilitiesRevokeSet struct {
	linuxCapabilitiesSetBase
}

func NewLinuxCapabilitiesRevokeSet(caps ...string) (*LinuxCapabilitiesRevokeSet, error) {
	l := LinuxCapabilitiesRevokeSet{
		linuxCapabilitiesSetBase{
			linuxCapabilitiesSetValue{
				make([]LinuxCapability, len(caps)),
			},
		},
	}
	for i, c := range caps {
		l.linuxCapabilitiesSetBase.val.Set[i] = LinuxCapability(c)
	}
	if err := l.AssertValid(); err != nil {
		return nil, err
	}
	return &l, nil
}

func (l LinuxCapabilitiesRevokeSet) AsIsolator() (*Isolator, error) {
	b, err := json.Marshal(l.linuxCapabilitiesSetBase.val)
	if err != nil {
		return nil, err
	}
	rm := json.RawMessage(b)
	return &Isolator{
		Name:     LinuxCapabilitiesRevokeSetName,
		ValueRaw: &rm,
		value:    &l,
	}, nil
}

type LinuxSeccompSet interface {
	Set() []LinuxSeccompEntry
	Errno() LinuxSeccompErrno
	AssertValid() error
}

type LinuxSeccompEntry string
type LinuxSeccompErrno string

type linuxSeccompValue struct {
	Set   []LinuxSeccompEntry `json:"set"`
	Errno LinuxSeccompErrno   `json:"errno"`
}

type linuxSeccompBase struct {
	val linuxSeccompValue
}

func (l linuxSeccompBase) multipleAllowed() bool {
	return false
}

func (l linuxSeccompBase) AssertValid() error {
	if len(l.val.Set) == 0 {
		return errors.New("set must be non-empty")
	}
	if l.val.Errno == "" {
		return nil
	}
	for _, c := range l.val.Errno {
		if !unicode.IsUpper(c) {
			return errors.New("errno must be an upper case string")
		}
	}
	return nil
}

func (l *linuxSeccompBase) UnmarshalJSON(b []byte) error {
	var v linuxSeccompValue
	err := json.Unmarshal(b, &v)
	if err != nil {
		return err
	}
	l.val = v
	return nil
}

func (l linuxSeccompBase) Set() []LinuxSeccompEntry {
	return l.val.Set
}

func (l linuxSeccompBase) Errno() LinuxSeccompErrno {
	return l.val.Errno
}

type LinuxSeccompRetainSet struct {
	linuxSeccompBase
}

func (l LinuxSeccompRetainSet) Conflicts() []ACIdentifier {
	return []ACIdentifier{LinuxSeccompRemoveSetName}
}

func NewLinuxSeccompRetainSet(errno string, syscall ...string) (*LinuxSeccompRetainSet, error) {
	l := LinuxSeccompRetainSet{
		linuxSeccompBase{
			linuxSeccompValue{
				make([]LinuxSeccompEntry, len(syscall)),
				LinuxSeccompErrno(errno),
			},
		},
	}
	for i, c := range syscall {
		l.linuxSeccompBase.val.Set[i] = LinuxSeccompEntry(c)
	}
	if err := l.AssertValid(); err != nil {
		return nil, err
	}
	return &l, nil
}

func (l LinuxSeccompRetainSet) AsIsolator() (*Isolator, error) {
	b, err := json.Marshal(l.linuxSeccompBase.val)
	if err != nil {
		return nil, err
	}
	rm := json.RawMessage(b)
	return &Isolator{
		Name:     LinuxSeccompRetainSetName,
		ValueRaw: &rm,
		value:    &l,
	}, nil
}

type LinuxSeccompRemoveSet struct {
	linuxSeccompBase
}

func (l LinuxSeccompRemoveSet) Conflicts() []ACIdentifier {
	return []ACIdentifier{LinuxSeccompRetainSetName}
}

func NewLinuxSeccompRemoveSet(errno string, syscall ...string) (*LinuxSeccompRemoveSet, error) {
	l := LinuxSeccompRemoveSet{
		linuxSeccompBase{
			linuxSeccompValue{
				make([]LinuxSeccompEntry, len(syscall)),
				LinuxSeccompErrno(errno),
			},
		},
	}
	for i, c := range syscall {
		l.linuxSeccompBase.val.Set[i] = LinuxSeccompEntry(c)
	}
	if err := l.AssertValid(); err != nil {
		return nil, err
	}
	return &l, nil
}

func (l LinuxSeccompRemoveSet) AsIsolator() (*Isolator, error) {
	b, err := json.Marshal(l.linuxSeccompBase.val)
	if err != nil {
		return nil, err
	}
	rm := json.RawMessage(b)
	return &Isolator{
		Name:     LinuxSeccompRemoveSetName,
		ValueRaw: &rm,
		value:    &l,
	}, nil
}
