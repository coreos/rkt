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

package image

import (
	"fmt"
	"sort"
	"strings"

	"github.com/appc/spec/schema/types"
)

func getLabelPriority(name types.ACIdentifier) int {
	labelsPriority := map[types.ACIdentifier]int{
		"version": 0,
		"os":      1,
		"arch":    2,
	}
	if i, ok := labelsPriority[name]; ok {
		return i
	}
	return len(labelsPriority) + 1
}

// labelsSlice implements sort.Interface for types.Labels
type labelsSlice types.Labels

func (p labelsSlice) Len() int { return len(p) }
func (p labelsSlice) Less(i, j int) bool {
	pi := getLabelPriority(p[i].Name)
	pj := getLabelPriority(p[j].Name)
	if pi != pj {
		return pi < pj
	}
	return p[i].Name < p[j].Name
}

func (p labelsSlice) Swap(i, j int) { p[i], p[j] = p[j], p[i] }

func labelsToString(inLabels types.Labels) string {
	// take a copy to avoid changing the original slice
	labels := append(types.Labels(nil), inLabels...)
	sort.Sort(labelsSlice(labels))

	var out []string
	for _, l := range labels {
		out = append(out, fmt.Sprintf("%q:%q", l.Name, l.Value))
	}
	return "[" + strings.Join(out, ", ") + "]"
}
