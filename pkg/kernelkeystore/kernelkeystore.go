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

// Package keystore implements the ACI keystore.
package kernelkeystore

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/jandre/keyutils"
	"github.com/coreos/rkt/Godeps/_workspace/src/golang.org/x/crypto/openpgp"
)

// A Config structure is used to configure a Keystore.
type Config struct {
	SystemKeyring string
	LocalKeyring  string
}

// A Keystore represents a repository of trusted public keys which can be
// used to verify PGP signatures.
type Keystore struct {
	*Config
}

// New returns a new Keystore based on config.
func New(config *Config) *Keystore {
	if config == nil {
		config = defaultConfig
	}
	return &Keystore{config}
}

func NewConfig(systemKeyring string, localKeyring string) *Config {
	return &Config{
		SystemKeyring: systemKeyring,
		LocalKeyring:  localKeyring,
	}
}

var defaultConfig = NewConfig("SystemRktKeys", "LocalRktKeys")

// CheckSignature is a convenience method for creating a Keystore with a default
// configuration and invoking CheckSignature.
func CheckSignature(prefix string, signed, signature io.Reader) (*openpgp.Entity, error) {
	ks := New(defaultConfig)
	return checkSignature(ks, prefix, signed, signature)
}

// CheckSignature takes a signed file and a detached signature and returns the signer
// if the signature is signed by a trusted signer.
// If the signer is unknown or not trusted, opengpg.ErrUnknownIssuer is returned.
func (ks *Keystore) CheckSignature(prefix string, signed, signature io.Reader) (*openpgp.Entity, error) {
	return checkSignature(ks, prefix, signed, signature)
}

func checkSignature(ks *Keystore, prefix string, signed, signature io.Reader) (*openpgp.Entity, error) {
	keyring, err := ks.loadKeyring(prefix)
	if err != nil {
		return nil, fmt.Errorf("keystore: error loading keyring %v", err)
	}
	entities, err := openpgp.CheckArmoredDetachedSignature(keyring, signed, signature)
	if err == io.EOF {
		// otherwise, the client failure is just "EOF", which is not helpful
		return nil, fmt.Errorf("keystore: no signatures found")
	}
	return entities, err
}

func (ks *Keystore) findKeyring(namespace string, prefix string) (keyutils.KeySerial, error) {
	var keyringname string
	keydata, err := ioutil.ReadFile("/proc/keys")
	if err != nil {
		return 0, err
	}
	if prefix != "" {
		keyringname = fmt.Sprint(namespace, "_", prefix)
	} else {
		keyringname = namespace
	}
	keys := strings.Split(string(keydata), "\n")
	for _, key := range keys {
		data := strings.Fields(key)
		if len(data) == 0 {
			continue
		}
		keyid, err := strconv.ParseInt(data[0], 16, 32)
		if err != nil {
			return 0, err
		}
		state := data[3]
		uid, err := strconv.Atoi(data[5])
		if err != nil {
			return 0, err
		}
		keytype := data[7]
		name := strings.Replace(data[8], ":", "", 1)

		if keytype == "keyring" && name == keyringname && state == "perm" && uid == os.Getuid() {
			return keyutils.KeySerial(keyid), nil
		}
	}
	return 0, fmt.Errorf("Unable to locate keyring")
}

func (ks *Keystore) newKeyring(prefix string) (keyutils.KeySerial, error) {
	keyringname := ""
	if prefix != "" {
		keyringname = fmt.Sprint(ks.Config.LocalKeyring, "_", prefix)
	} else {
		keyringname = ks.Config.LocalKeyring
	}
	return keyutils.NewKeyRing(keyringname, keyutils.KEY_SPEC_USER_KEYRING)
}

// DeleteTrustedKeyPrefix deletes the prefix trusted key identified by fingerprint.
func (ks *Keystore) DeleteTrustedKeyPrefix(prefix string, fingerprint string) error {
	keyring, err := ks.findKeyring(ks.Config.LocalKeyring, prefix)
	if err != nil {
		return err
	}
	keys, err := keyutils.ListKeysInKeyRing(keyring)
	if err != nil {
		return err
	}
	for _, key := range keys {
		if key.Description == fingerprint {
			return keyutils.Revoke(key.Serial)
		}
	}
	return nil
}

// MaskTrustedKeySystemPrefix masks the system prefix trusted key identified by fingerprint.
func (ks *Keystore) MaskTrustedKeySystemPrefix(prefix, fingerprint string) (keyutils.KeySerial, error) {
	keyring, err := ks.findKeyring(ks.Config.LocalKeyring, prefix)
	if keyring == 0 {
		keyring, err = ks.newKeyring(prefix)
	}
	if err != nil {
		return 0, err
	}
	return keyutils.AddKey(keyutils.USER, fingerprint, "dummy", keyring)
}

// DeleteTrustedKeyRoot deletes the root trusted key identified by fingerprint.
func (ks *Keystore) DeleteTrustedKeyRoot(fingerprint string) error {
	return ks.DeleteTrustedKeyPrefix("", fingerprint)
}

// MaskTrustedKeySystemRoot masks the system root trusted key identified by fingerprint.
func (ks *Keystore) MaskTrustedKeySystemRoot(fingerprint string) (keyutils.KeySerial, error) {
	return ks.MaskTrustedKeySystemPrefix("", fingerprint)
}

// StoreTrustedKeyPrefix stores the contents of public key r as a prefix trusted key.
func (ks *Keystore) StoreTrustedKeyPrefix(prefix string, r io.Reader) (keyutils.KeySerial, error) {
	return ks.storeTrustedKey(prefix, r)
}

// StoreTrustedKeyRoot stores the contents of public key r as a root trusted key.
func (ks *Keystore) StoreTrustedKeyRoot(r io.Reader) (keyutils.KeySerial, error) {
	return ks.storeTrustedKey("", r)
}

func (ks *Keystore) storeTrustedKey(prefix string, r io.Reader) (keyutils.KeySerial, error) {
	pubkeyBytes, err := ioutil.ReadAll(r)
	pubkeyString := string(pubkeyBytes)
	if err != nil {
		return 0, err
	}
	entityList, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(pubkeyBytes))
	if err != nil {
		return 0, err
	}
	keyring, err := ks.findKeyring(ks.Config.LocalKeyring, prefix)
	if keyring == 0 {
		keyring, err = ks.newKeyring(prefix)
	}
	if err != nil {
		return 0, err
	}
	fingerprint := fmt.Sprintf("%x", entityList[0].PrimaryKey.Fingerprint)
	return keyutils.AddKey(keyutils.USER, fingerprint, pubkeyString, keyutils.KeySerial(keyring))
}

func entityFromData(fingerprint string, data []byte) (*openpgp.Entity, error) {
	entityList, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if len(entityList) < 1 {
		return nil, errors.New("missing opengpg entity")
	}
	if fingerprint != fmt.Sprintf("%x", entityList[0].PrimaryKey.Fingerprint) {
		return nil, fmt.Errorf("fingerprint mismatch: %q:%q", fingerprint, fmt.Sprintf("%x", entityList[0].PrimaryKey.Fingerprint))
	}
	return entityList[0], nil
}

func readKeysFromKeyring(keyring keyutils.KeySerial) ([]*keyutils.KeyDesc, error) {
	return keyutils.ListKeysInKeyRing(keyring)
}

func (ks *Keystore) readKeysByNamespace(namespace string, prefix string) []*keyutils.KeyDesc {
	var trustedKeys []*keyutils.KeyDesc
	for {
		keyring, err := ks.findKeyring(namespace, prefix)
		if err == nil {
			keys, err := readKeysFromKeyring(keyring)
			if err == nil {
				trustedKeys = append(trustedKeys, keys...)
			}
		}
		if prefix == "" {
			break
		}
		offset := strings.LastIndex(prefix, "/")
		if offset == -1 {
			prefix = ""
		} else {
			prefix = prefix[:strings.LastIndex(prefix, "/")]
		}
	}
	return trustedKeys
}

func (ks *Keystore) readKeys(prefix string) []*keyutils.KeyDesc {
	var trusted_keys []*keyutils.KeyDesc
	keys := ks.readKeysByNamespace(ks.Config.SystemKeyring, prefix)
	trusted_keys = append(trusted_keys, keys...)
	keys = ks.readKeysByNamespace(ks.Config.LocalKeyring, prefix)
	trusted_keys = append(trusted_keys, keys...)
	return trusted_keys
}

func (ks *Keystore) loadKeyring(prefix string) (openpgp.KeyRing, error) {
	var keyring openpgp.EntityList
	trustedKeys := make(map[string]*openpgp.Entity)
	keys := ks.readKeys(prefix)
	for _, key := range keys {
		keydata, err := keyutils.ReadKey(key.Serial)
		if err != nil {
			return nil, err
		}
		if keydata == "dummy" {
			delete(trustedKeys, key.Description)
		} else {
			entity, err := entityFromData(key.Description, []byte(keydata))
			if err != nil {
				return nil, err
			}
			trustedKeys[key.Description] = entity
		}
	}
	for _, v := range trustedKeys {
		keyring = append(keyring, v)
	}
	return keyring, nil
}

// NewTestKeystore creates a new KeyStore backed by a temp directory.
// NewTestKeystore returns a KeyStore, the path to the temp directory, and
// an error if any.
func NewTestKeystore() (*Keystore, error) {
	c := NewConfig("TestSystemRktKeys", "TestLocalRktKeys")
	return New(c), nil
}
