// Copyright 2016 The appc Authors
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

package log

import (
	"fmt"
	"io"
	"os"
	"strings"
)

var debugEnabled bool

func printTo(w io.Writer, i ...interface{}) {
	s := fmt.Sprint(i...)
	fmt.Fprintln(w, strings.TrimSuffix(s, "\n"))
}

// Info prints a message to stderr.
func Info(i ...interface{}) {
	printTo(os.Stderr, i...)
}

// Debug prints a message to stderr if debug is enabled.
func Debug(i ...interface{}) {
	if debugEnabled {
		printTo(os.Stderr, i...)
	}
}

// InitDebug enables debug output.
func InitDebug() {
	debugEnabled = true
}
