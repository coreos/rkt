// Copyright 2016 CNI authors
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

package testutils

import (
	"encoding/json"
	"io/ioutil"
	"os"

	"github.com/containernetworking/cni/pkg/types"
)

func envCleanup() {
	os.Unsetenv("CNI_COMMAND")
	os.Unsetenv("CNI_PATH")
	os.Unsetenv("CNI_NETNS")
	os.Unsetenv("CNI_IFNAME")
}

func CmdAddWithResult(cniNetns, cniIfname string, f func() error) (*types.Result, error) {
	os.Setenv("CNI_COMMAND", "ADD")
	os.Setenv("CNI_PATH", os.Getenv("PATH"))
	os.Setenv("CNI_NETNS", cniNetns)
	os.Setenv("CNI_IFNAME", cniIfname)
	defer envCleanup()

	// Redirect stdout to capture plugin result
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		return nil, err
	}

	os.Stdout = w
	err = f()
	w.Close()
	if err != nil {
		return nil, err
	}

	// parse the result
	out, err := ioutil.ReadAll(r)
	os.Stdout = oldStdout
	if err != nil {
		return nil, err
	}

	result := types.Result{}
	err = json.Unmarshal(out, &result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func CmdDelWithResult(cniNetns, cniIfname string, f func() error) error {
	os.Setenv("CNI_COMMAND", "DEL")
	os.Setenv("CNI_PATH", os.Getenv("PATH"))
	os.Setenv("CNI_NETNS", cniNetns)
	os.Setenv("CNI_IFNAME", cniIfname)
	defer envCleanup()

	return f()
}
