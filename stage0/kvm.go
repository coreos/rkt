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

package stage0

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	kvmSettingsDir        = "/kvm"
	kvmPrivateKeyFilename = "/ssh_kvm_key"
)

// fileAccessible checks if the given path exists and is a regular file
func fileAccessible(path string) bool {
	info, err := os.Stat(path)
	if err == nil {
		return info.Mode().IsRegular()
	}
	return false
}

func sshPrivateKeyPath(dataDir string) string {
	return filepath.Join(dataDir, kvmSettingsDir, kvmPrivateKeyFilename)
}

func sshPublicKeyPath(dataDir string) string {
	return filepath.Join(dataDir, kvmSettingsDir, kvmPrivateKeyFilename+".pub")
}

func createKvmSettingsDir(dataDir string) error {
	return os.MkdirAll(filepath.Join(dataDir, kvmSettingsDir), 0700)
}

// generateKeyPair calls ssh-keygen with private key location for key generation purpose
func generateKeyPair(private string) error {
	out, err := exec.Command(
		"ssh-keygen",
		"-q",        // silence
		"-t", "dsa", // type
		"-b", "1024", // length in bits
		"-f", private, // output file
		"-N", "", // no passphrase
	).Output()
	if err != nil {
		// out is in form of bytes buffer and we have to turn it into slice ending on first \0 occurence
		return fmt.Errorf("error in keygen time. ret_val: %v, output: %v", err, string(out[:len(out)]))
	}
	return nil
}

func ensureKeysExistOnHost(dataDir string) error {
	private, public := sshPrivateKeyPath(dataDir), sshPublicKeyPath(dataDir)
	if !fileAccessible(private) || !fileAccessible(public) {
		err := createKvmSettingsDir(dataDir)
		if err != nil {
			return err
		}

		err = generateKeyPair(private)
		if err != nil {
			return err
		}
	}
	return nil
}

func ensureAuthorizedKeysExist(keyDirPath, dataDir string) error {
	fout, err := os.OpenFile(
		filepath.Join(keyDirPath, "/authorized_keys"),
		os.O_CREATE|os.O_TRUNC|os.O_WRONLY,
		0600,
	)
	if err != nil {
		return err
	}
	defer fout.Close()

	fin, err := os.Open(sshPublicKeyPath(dataDir))
	if err != nil {
		return err
	}
	defer fin.Close()

	if _, err = io.Copy(fout, fin); err != nil {
		return err
	}
	return fout.Sync()
}

func ensureKeysExistInPod(podRootfsPath, dataDir string) error {
	keyDirPath := filepath.Join(podRootfsPath, "/root", "/.ssh")
	if err := os.MkdirAll(keyDirPath, 0700); err != nil {
		return err
	}
	return ensureAuthorizedKeysExist(keyDirPath, dataDir)
}

func kvmCheckSSHSetup(rootfsPath, dataDir string) error {
	if err := ensureKeysExistOnHost(dataDir); err != nil {
		return err
	}

	return ensureKeysExistInPod(rootfsPath, dataDir)
}
