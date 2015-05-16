// Copyright 2014 The rkt Authors
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

package main

import "fmt"

// pflag type that accumulates flags
type Multi struct {
	v    []string
	name string
}

func (m *Multi) String() string {
	return fmt.Sprintf("%v", m.v)
}

func (m *Multi) Set(s string) error {
	m.v = append(m.v, s)
	return nil
}
func (m *Multi) Type() string {
	return m.name
}

func NewMulti(t string) *Multi {
	return &Multi{
		v:    []string{},
		name: t,
	}
}

// pflag type that accumulates optional flags where the optional flag depends on preceding pivoting flag
// e.g. cmd -i IMG1 -i IMG2 -s IMG2_OPTS -i IMG3 -i IMG4 -s IMG4_OPTS
// v is a map (poormans sparse array) v[1]=IMG2_OPTS v[3]=IMG4_OPTS
type Buddy struct {
	v     map[int]string
	other *Multi
	name  string
}

func (b *Buddy) Set(s string) error {
	if len(b.other.v) < 1 {
		return fmt.Errorf("%s flag needs to follow %s", b.Type(), b.other.Type())
	}
	ix := len(b.other.v) - 1
	b.v[ix] = s
	return nil
}

func (b *Buddy) String() string {
	return fmt.Sprintf("%+v", b.v)
}
func (b *Buddy) Type() string {
	return b.name
}

func NewBuddy(m *Multi, t string) *Buddy {
	return &Buddy{
		v:     make(map[int]string),
		other: m,
		name:  t,
	}
}
