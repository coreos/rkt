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

package oci2aci

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const (
	// Path to config file inside the bundle
	ConfigFile  = "config.json"
	RuntimeFile = "runtime.json"
	// Path to rootfs directory inside the bundle
	RootfsDir = "rootfs"
)

var (
	ErrNoRootFS = errors.New("no rootfs found in bundle")
	ErrNoConfig = errors.New("no config json file found in bundle")
	ErrNoRun    = errors.New("no runtime json file found in bundle")
)

type validateRes struct {
	cfgOK   bool
	runOK   bool
	rfsOK   bool
	config  io.Reader
	runtime io.Reader
}

func validateOCIProc(path string) bool {
	var bRes bool
	err := validateBundle(path)
	if err != nil {
		if debugEnabled {
			log.Printf("%s: invalid oci bundle: %v\n", path, err)
		}
		bRes = false
	} else {
		if debugEnabled {
			log.Printf("%s: valid oci bundle\n", path)
		}
		bRes = true
	}
	return bRes
}

func validateBundle(path string) error {
	fi, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("error accessing bundle: %v", err)
	}
	if !fi.IsDir() {
		return fmt.Errorf("given path %q is not a directory", path)
	}
	var flist []string
	var res validateRes
	walkBundle := func(fpath string, fi os.FileInfo, err error) error {
		rpath, err := filepath.Rel(path, fpath)
		if err != nil {
			return err
		}
		switch rpath {
		case ".":
		case ConfigFile:
			res.config, err = os.Open(fpath)
			if err != nil {
				return err
			}
			res.cfgOK = true
		case RuntimeFile:
			res.runtime, err = os.Open(fpath)
			if err != nil {
				return err
			}
			res.runOK = true
		case RootfsDir:
			if !fi.IsDir() {
				return errors.New("rootfs is not a directory")
			}
			res.rfsOK = true
		default:
			flist = append(flist, rpath)
		}
		return nil
	}
	if err := filepath.Walk(path, walkBundle); err != nil {
		return err
	}
	return checkBundle(res, flist)
}

func checkBundle(res validateRes, files []string) error {
	defer func() {
		if rc, ok := res.config.(io.Closer); ok {
			rc.Close()
		}
		if rc, ok := res.runtime.(io.Closer); ok {
			rc.Close()
		}
	}()
	if !res.cfgOK {
		return ErrNoConfig
	}
	if !res.runOK {
		return ErrNoRun
	}
	if !res.rfsOK {
		return ErrNoRootFS
	}
	_, err := ioutil.ReadAll(res.config)
	if err != nil {
		return fmt.Errorf("error reading the bundle: %v", err)
	}
	_, err = ioutil.ReadAll(res.runtime)
	if err != nil {
		return fmt.Errorf("error reading the bundle: %v", err)
	}

	for _, f := range files {
		if !strings.HasPrefix(f, "rootfs") {
			return fmt.Errorf("unrecognized file path in bundle: %q", f)
		}
	}
	return nil
}
