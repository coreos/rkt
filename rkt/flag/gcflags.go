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

package flag

const (
	gcNone = 0
	gcNet  = 1 << (iota - 1)

	gcAll = (gcNet)
)

var (
	gcOptions = []string{"none", "net", "all"}

	gcOptionsMap = map[string]int{
		gcOptions[0]: gcNone,
		gcOptions[1]: gcNet,
		gcOptions[2]: gcAll,
	}
)

type GcFlags struct {
	*bitFlags
}

func NewGcFlags(defOpts string) (*GcFlags, error) {
	bf, err := newBitFlags(gcOptions, defOpts, gcOptionsMap)
	if err != nil {
		return nil, err
	}

	sf := &GcFlags{
		bitFlags: bf,
	}
	return sf, nil
}

func (sf *GcFlags) GcNet() bool {
	return sf.hasFlag(gcNet)
}

func (sf *GcFlags) GcAll() bool {
	return sf.hasFlag(gcAll)
}

func (sf *GcFlags) GcNone() bool {
	return sf.flags == gcNone
}
