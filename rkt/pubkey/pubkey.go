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

package pubkey

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/coreos/rkt/pkg/keystore"
	"github.com/coreos/rkt/rkt/config"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/discovery"
	"github.com/coreos/rkt/Godeps/_workspace/src/golang.org/x/crypto/openpgp"
)

type Manager struct {
	AuthPerHost        map[string]config.Headerer
	InsecureAllowHttp  bool
	TrustKeysFromHttps bool
	Ks                 *keystore.Keystore
	Debug              bool
}

type AcceptOption int
type OverrideOption int

const (
	AcceptForce AcceptOption = iota
	AcceptAsk
)

const (
	OverrideAllow OverrideOption = iota
	OverrideDeny
)

// GetPubKeyLocations discovers one location at prefix
func (m *Manager) GetPubKeyLocations(prefix string) ([]string, error) {
	if prefix == "" {
		return nil, fmt.Errorf("empty prefix")
	}

	kls, err := m.metaDiscoverPubKeyLocations(prefix)
	if err != nil {
		return nil, fmt.Errorf("prefix meta discovery error: %v", err)
	}

	if len(kls) == 0 {
		return nil, fmt.Errorf("meta discovery on %s resulted in no keys", prefix)
	}

	return kls, nil
}

// AddKeys adds the keys listed in pkls at prefix
func (m *Manager) AddKeys(pkls []string, prefix string, accept AcceptOption, override OverrideOption) error {
	if m.Ks == nil {
		return fmt.Errorf("no keystore available to add keys to")
	}

	for _, pkl := range pkls {
		u, err := url.Parse(pkl)
		if err != nil {
			return err
		}
		pk, err := m.getPubKey(u)
		if err != nil {
			return fmt.Errorf("error accessing the key %s: %v", pkl, err)
		}
		defer pk.Close()

		exists, err := m.Ks.TrustedKeyPrefixExists(prefix, pk)
		if err != nil {
			return fmt.Errorf("error reading the key %s: %v", pkl, err)
		}
		err = displayKey(prefix, pkl, pk)
		if err != nil {
			return fmt.Errorf("error displaying the key %s: %v", pkl, err)
		}
		if exists && override == OverrideDeny {
			stderr("Key %q already in the keystore", pkl)
			continue
		}

		if m.TrustKeysFromHttps && u.Scheme == "https" {
			accept = AcceptForce
		}

		if accept == AcceptAsk {
			accepted, err := reviewKey()
			if err != nil {
				return fmt.Errorf("error reviewing key: %v", err)
			}
			if !accepted {
				stderr("Not trusting %q", pkl)
				continue
			}
		}

		if accept == AcceptForce {
			stderr("Trusting %q for prefix %q without fingerprint review.", pkl, prefix)
		} else {
			stderr("Trusting %q for prefix %q after fingerprint review.", pkl, prefix)
		}

		if prefix == "" {
			path, err := m.Ks.StoreTrustedKeyRoot(pk)
			if err != nil {
				return fmt.Errorf("Error adding root key: %v", err)
			}
			stderr("Added root key at %q", path)
		} else {
			path, err := m.Ks.StoreTrustedKeyPrefix(prefix, pk)
			if err != nil {
				return fmt.Errorf("Error adding key for prefix %q: %v", prefix, err)
			}
			stderr("Added key for prefix %q at %q", prefix, path)
		}
	}
	return nil
}

// metaDiscoverPubKeyLocations discovers the public key through ACDiscovery by applying prefix as an ACApp
func (m *Manager) metaDiscoverPubKeyLocations(prefix string) ([]string, error) {
	app, err := discovery.NewAppFromString(prefix)
	if err != nil {
		return nil, err
	}

	hostHeaders := config.ResolveAuthPerHost(m.AuthPerHost)
	ep, attempts, err := discovery.DiscoverPublicKeys(*app, hostHeaders, m.InsecureAllowHttp)
	if err != nil {
		return nil, err
	}

	if m.Debug {
		for _, a := range attempts {
			stderr("meta tag 'ac-discovery-pubkeys' not found on %s: %v", a.Prefix, a.Error)
		}
	}

	return ep.Keys, nil
}

// getPubKey retrieves a public key (if remote), and verifies it's a gpg key
func (m *Manager) getPubKey(u *url.URL) (*os.File, error) {
	switch u.Scheme {
	case "":
		return os.Open(u.Path)
	case "http":
		if !m.InsecureAllowHttp {
			return nil, fmt.Errorf("--insecure-allow-http required for http URLs")
		}
		fallthrough
	case "https":
		return downloadKey(u)
	}

	return nil, fmt.Errorf("only local files and http or https URLs supported")
}

// downloadKey retrieves the file, storing it in a deleted tempfile
func downloadKey(u *url.URL) (*os.File, error) {
	tf, err := ioutil.TempFile("", "")
	if err != nil {
		return nil, fmt.Errorf("error creating tempfile: %v", err)
	}
	os.Remove(tf.Name()) // no need to keep the tempfile around

	defer func() {
		if tf != nil {
			tf.Close()
		}
	}()

	// TODO(krnowak): we should probably apply credential headers
	// from config here
	res, err := http.Get(u.String())
	if err != nil {
		return nil, fmt.Errorf("error getting key: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad HTTP status code: %d", res.StatusCode)
	}

	if _, err := io.Copy(tf, res.Body); err != nil {
		return nil, fmt.Errorf("error copying key: %v", err)
	}

	if _, err = tf.Seek(0, os.SEEK_SET); err != nil {
		return nil, fmt.Errorf("error seeking: %v", err)
	}

	retTf := tf
	tf = nil
	return retTf, nil
}

// displayKey shows the key summary
func displayKey(prefix, location string, key *os.File) error {
	defer key.Seek(0, os.SEEK_SET)

	kr, err := openpgp.ReadArmoredKeyRing(key)
	if err != nil {
		return fmt.Errorf("error reading key: %v", err)
	}

	stderr("prefix: %q\nkey: %q", prefix, location)
	for _, k := range kr {
		stderr("gpg key fingerprint is: %s", fingerToString(k.PrimaryKey.Fingerprint))
		for _, sk := range k.Subkeys {
			stderr("    subkey fingerprint: %s", fingerToString(sk.PublicKey.Fingerprint))
		}
		for n, _ := range k.Identities {
			stderr("\t%s", n)
		}
	}
	return nil
}

// reviewKey asks the user to accept the key
func reviewKey() (bool, error) {
	in := bufio.NewReader(os.Stdin)
	for {
		stderr("Are you sure you want to trust this key (yes/no)?")
		input, err := in.ReadString('\n')
		if err != nil {
			return false, fmt.Errorf("error reading input: %v", err)
		}
		switch input {
		case "yes\n":
			return true, nil
		case "no\n":
			return false, nil
		default:
			stderr("Please enter 'yes' or 'no'")
		}
	}
}

func fingerToString(fpr [20]byte) string {
	str := ""
	for i, b := range fpr {
		if i > 0 && i%2 == 0 {
			str += " "
			if i == 10 {
				str += " "
			}
		}
		str += strings.ToUpper(fmt.Sprintf("%.2x", b))
	}
	return str
}

func stderr(format string, a ...interface{}) {
	out := fmt.Sprintf(format, a...)
	fmt.Fprintln(os.Stderr, strings.TrimSuffix(out, "\n"))
}
