// Copyright 2015 CoreOS, Inc.
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

//+build linux

// implements https://github.com/coreos/rocket/issues/367

package main

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/coreos/rocket/Godeps/_workspace/src/github.com/appc/spec/discovery"
	"github.com/coreos/rocket/Godeps/_workspace/src/golang.org/x/crypto/openpgp"
	"github.com/coreos/rocket/pkg/keystore"
)

var (
	cmdTrust = &Command{
		Name:    cmdTrustName,
		Summary: "Trust a key for image verification",
		Usage:   "[--prefix PREFIX] [--root] [PUBKEY ...]",
		Description: `Adds keys to the local keystore for use in verifying signed images.
PUBKEY may be either a local file or URL,
PREFIX scopes the applicability of PUBKEY to image names sharing PREFIX.
Meta discovery of PUBKEY at PREFIX will be attempted if no PUBKEY is specified.
--root must be specified to add keys with no prefix.`,
		Run: runTrust,
	}
	flagPrefix string
	flagRoot   bool
)

const (
	cmdTrustName = "trust"
)

func init() {
	commands = append(commands, cmdTrust)
	cmdTrust.Flags.StringVar(&flagPrefix, "prefix", "", "prefix to limit trust to")
	cmdTrust.Flags.BoolVar(&flagRoot, "root", false, "add root key without a prefix")
}

func runTrust(args []string) (exit int) {
	if flagPrefix == "" && !flagRoot {
		if len(args) != 0 {
			stderr("--root required for non-prefixed (root) keys")
		} else {
			printCommandUsageByName(cmdTrustName)
		}
		return 1
	}

	if flagPrefix != "" && flagRoot {
		stderr("--root and --prefix usage mutually exclusive")
		return 1
	}

	pkls, err := getPubKeyLocations(flagPrefix, args)
	if err != nil {
		stderr("Error determining key location: %v", err)
		return 1
	}

	if err := addKeys(pkls, flagPrefix); err != nil {
		stderr("Error adding keys: %v", err)
		return 1
	}

	return 0
}

// addKeys adds the keys listed in pkls at prefix
func addKeys(pkls []string, prefix string) error {
	for _, pkl := range pkls {
		pk, err := getPubKey(pkl)
		if err != nil {
			return fmt.Errorf("error accessing key: %v", err)
		}
		defer pk.Close()

		accepted, err := reviewKey(prefix, pkl, pk, globalFlags.InsecureSkipVerify)
		if err != nil {
			return fmt.Errorf("error reviewing key: %v", err)
		}

		if !accepted {
			stdout("Not trusting %q", pkl)
			continue
		}

		stdout("Trusting %q for prefix %q.", pkl, flagPrefix)

		if err := addPubKey(prefix, pk); err != nil {
			return fmt.Errorf("Error adding key: %v", err)
		}
	}
	return nil
}

// addPubKey adds a key to the keystore
func addPubKey(prefix string, key *os.File) (err error) {
	ks := keystore.New(nil)

	var path string
	if prefix == "" {
		path, err = ks.StoreTrustedKeyRoot(key)
		stdout("Added root key at %q", path)
	} else {
		path, err = ks.StoreTrustedKeyPrefix(prefix, key)
		stdout("Added key for prefix %q at %q", prefix, path)
	}

	return
}

// getPubKeyLocation either returns the location supplied in argv or discovers one @ prefix
func getPubKeyLocations(prefix string, args []string) ([]string, error) {
	if len(args) > 0 {
		return args, nil
	}

	if prefix == "" {
		return nil, fmt.Errorf("at least one key or --prefix required")
	}

	kls, err := metaDiscoverPubKeyLocations(prefix)
	if err != nil {
		return nil, fmt.Errorf("--prefix meta discovery error: %v", err)
	}

	return kls, nil
}

// metaDiscoverPubKeyLocations discovers the public key through ACDiscovery by applying prefix as an ACApp
func metaDiscoverPubKeyLocations(prefix string) ([]string, error) {
	app, err := discovery.NewAppFromString(prefix)
	if err != nil {
		return nil, err
	}

	ep, err := discovery.DiscoverEndpoints(*app, true)
	if err != nil {
		return nil, err
	}

	return ep.Keys, nil
}

// getPubKey retrieves a public key (if remote), and verifies it's a gpg key
func getPubKey(location string) (*os.File, error) {
	u, err := url.Parse(location)
	if err != nil {
		return nil, err
	}

	switch u.Scheme {
	case "":
		return os.Open(location)
	case "http", "https":
		return downloadKey(u.String())
	}

	return nil, fmt.Errorf("only http and https urls supported")
}

// downloadKey retrieves the file, storing it in a deleted tempfile
func downloadKey(url string) (*os.File, error) {
	tf, err := ioutil.TempFile("", "")
	if err != nil {
		return nil, fmt.Errorf("error creating tempfile: %v", err)
	}
	os.Remove(tf.Name()) // no need to keep the tempfile around

	defer func() {
		if err != nil {
			tf.Close()
		}
	}()

	res, err := http.Get(url)
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

	tf.Seek(0, os.SEEK_SET)

	return tf, nil
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

// reviewKey shows the key summary and conditionally asks the user to accept it
func reviewKey(prefix string, location string, key *os.File, forceAccept bool) (bool, error) {
	defer key.Seek(0, os.SEEK_SET)

	kr, err := openpgp.ReadArmoredKeyRing(key)
	if err != nil {
		return false, fmt.Errorf("error reading key: %v", err)
	}

	stdout("Prefix: %q\nKey: %q", prefix, location)
	for _, k := range kr {
		stdout("GPG key fingerprint is: %s", fingerToString(k.PrimaryKey.Fingerprint))
		for _, sk := range k.Subkeys {
			stdout("    Subkey fingerprint: %s", fingerToString(sk.PublicKey.Fingerprint))
		}
		for n, _ := range k.Identities {
			stdout("\t%s", n)
		}
	}

	if !forceAccept {
		in := bufio.NewReader(os.Stdin)
		for {
			fmt.Printf("Are you sure you want to trust this key (yes/no)? ")
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
				stdout("Please enter 'yes' or 'no'")
			}
		}
	}
	return true, nil
}
