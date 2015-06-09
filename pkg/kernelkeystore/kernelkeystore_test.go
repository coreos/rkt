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

// The keystore tests require opengpg keys from the keystoretest package (keystoretest.KeyMap).
// The opengpg keys are auto generated by running the keygen.go command.
// keygen.go should not be run by an automated process. keygen.go is a helper to generate
// the keystoretest/keymap.go source file.
//
// If additional opengpg keys are need for testing, please use the following process:
//   * add a new key name to keygen.go
//   * cd keystore/keystoretest
//   * go run keygen.go
//   * check in the results

package kernelkeystore

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/coreos/rkt/pkg/kernelkeystore/kernelkeystoretest"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/jandre/keyutils"
	"github.com/coreos/rkt/Godeps/_workspace/src/golang.org/x/crypto/openpgp/errors"
)

func TestStoreTrustedKey(t *testing.T) {
	ks, err := NewTestKeystore()
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}

	armoredPublicKey := keystoretest.KeyMap["example.com"].ArmoredPublicKey
	fingerprint := keystoretest.KeyMap["example.com"].Fingerprint

	output, err := ks.StoreTrustedKeyPrefix("example.com/foo", bytes.NewBufferString(armoredPublicKey))
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	key, err := keyutils.DescribeKey(output)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if key.Description != fingerprint {
		t.Errorf("expected finger print %s, got %v", fingerprint, key.Description)
	}
	if err := ks.DeleteTrustedKeyPrefix("example.com/foo", fingerprint); err != nil {
		t.Errorf("unexpected error %v", err)
	}
	output, err = ks.MaskTrustedKeySystemPrefix("example.com/foo", fingerprint)
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}
	keydata, err := keyutils.ReadKey(output)
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}
	if keydata != "dummy" {
		t.Errorf("unexpected key contents")
	}

	output, err = ks.StoreTrustedKeyRoot(bytes.NewBufferString(armoredPublicKey))
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	key, err = keyutils.DescribeKey(output)
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}
	if key.Description != fingerprint {
		t.Errorf("expected finger print %s, got %v", fingerprint, key.Description)
	}
	if err := ks.DeleteTrustedKeyRoot(fingerprint); err != nil {
		t.Errorf("unexpected error %v", err)
	}
	output, err = ks.MaskTrustedKeySystemRoot(fingerprint)
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}
	keydata, err = keyutils.ReadKey(output)
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}
	if keydata != "dummy" {
		t.Errorf("unexpected key contents")
	}
}

func TestCheckSignature(t *testing.T) {
	trustedPrefixKeys := []string{
		"example.com/app",
		"acme.com/services",
		"acme.com/services/web/nginx",
	}
	trustedRootKeys := []string{
		"coreos.com",
	}

	ks, err := NewTestKeystore()
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}

	for _, key := range trustedPrefixKeys {
		if _, err := ks.StoreTrustedKeyPrefix(key, bytes.NewBufferString(keystoretest.KeyMap[key].ArmoredPublicKey)); err != nil {
			t.Fatalf("unexpected error %v", err)
		}
	}
	for _, key := range trustedRootKeys {
		if _, err := ks.StoreTrustedKeyRoot(bytes.NewBufferString(keystoretest.KeyMap[key].ArmoredPublicKey)); err != nil {
			t.Fatalf("unexpected error %v", err)
		}
	}

	if _, err := ks.MaskTrustedKeySystemRoot(keystoretest.KeyMap["acme.com"].Fingerprint); err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	checkSignatureTests := []struct {
		name    string
		key     string
		trusted bool
	}{
		{"coreos.com/etcd", "coreos.com", true},
		{"coreos.com/fleet", "coreos.com", true},
		{"coreos.com/flannel", "coreos.com", true},
		{"example.com/app", "example.com/app", true},
		{"acme.com/services/web/nginx", "acme.com/services/web/nginx", true},
		{"acme.com/services/web/auth", "acme.com/services", true},
		{"acme.com/etcd", "acme.com", false},
		{"acme.com/web/nginx", "acme.com", false},
		{"acme.com/services/web", "acme.com/services/web/nginx", false},
	}
	for _, tt := range checkSignatureTests {
		key := keystoretest.KeyMap[tt.key]
		message, signature, err := keystoretest.NewMessageAndSignature(key.ArmoredPrivateKey)
		if err != nil {
			t.Fatalf("unexpected error %v", err)
			continue
		}
		signer, err := ks.CheckSignature(tt.name, message, signature)
		if tt.trusted {
			if err != nil {
				t.Errorf("unexpected error %v", err)
			}
			fingerprint := fmt.Sprintf("%x", signer.PrimaryKey.Fingerprint)
			if fingerprint != key.Fingerprint {
				t.Errorf("expected fingerprint == %v, got %v", key.Fingerprint, fingerprint)
			}
			continue
		}
		if err == nil {
			t.Errorf("expected ErrUnknownIssuer error")
			continue
		}
		if err.Error() != errors.ErrUnknownIssuer.Error() {
			t.Errorf("unexpected error %v", err)
		}
	}
}
