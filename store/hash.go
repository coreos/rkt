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

package store

import (
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"hash"
	"reflect"
	"strings"
)

type HashAlgorithm interface {
	// ValidateHashString checks that a given input hash string is valid
	ValidateHashString(hashStr string) error
	// NormalizeHashString, given an input hash string, returns the
	//normalized hash string depending on the hash algorithm implementation
	NormalizeHashString(hashStr string) (string, error)
	// HashToHashString takes a hash.Hash and returns a hash string. The
	// hash.Hash must be of the supported type.
	HashToHashString(h hash.Hash) (string, error)
	// NewHash returns a new hash.Hash of the supported hashAlgorithm type
	NewHash() hash.Hash
}

// SHA512HashAlgorithm is a hash algorithm that uses sha512 sums
// To ameliorate excessively long hash string, hash strings use only the first
// half of a sha512 rather than the entire hash.
type SHA512HashAlgorithm struct {
	hashPrefix              string
	lenHash                 int
	lenSum                  int
	lenHashString           int
	lenNormalizedSum        int
	lenNormalizedHashString int
	minlenHashString        int
}

func NewSHA512HashAlgorithm() *SHA512HashAlgorithm {
	hashPrefix := "sha512"
	lenHash := sha512.Size  // raw byte size
	lenSum := (lenHash) * 2 // in hex characters
	lenHashString := len(hashPrefix) + 1 + lenSum
	lenNormalizedSum := (lenHash / 2) * 2 // half length, in hex characters
	lenNormalizedHashString := len(hashPrefix) + 1 + lenNormalizedSum
	minlenHashString := len(hashPrefix) + 1 + 2 // at least sha512-aa

	return &SHA512HashAlgorithm{
		hashPrefix:              hashPrefix,
		lenHash:                 lenHash,
		lenSum:                  lenSum,
		lenHashString:           lenHashString,
		lenNormalizedSum:        lenNormalizedSum,
		lenNormalizedHashString: lenNormalizedHashString,
		minlenHashString:        minlenHashString,
	}
}

func (ha *SHA512HashAlgorithm) ValidateHashString(hashStr string) error {
	elems := strings.Split(hashStr, "-")
	if len(elems) != 2 {
		return fmt.Errorf("badly formatted hash string")
	}
	hashPrefix := elems[0]
	if hashPrefix != ha.hashPrefix {
		return fmt.Errorf("wrong hash string prefix %q", hashPrefix)
	}
	if len(hashStr) > ha.lenHashString {
		return fmt.Errorf("hash string too long")
	}
	if len(hashStr) < ha.minlenHashString {
		return fmt.Errorf("hash string too short")
	}
	return nil
}

// NormalizeHashString, given an input hashstring, returns the hash string
// cutted to lenNormalizedHashString if it's longer or an error if the hash
// string is not valid
func (ha *SHA512HashAlgorithm) NormalizeHashString(hashStr string) (string, error) {
	if err := ha.ValidateHashString(hashStr); err != nil {
		return "", err
	}
	if len(hashStr) > ha.lenNormalizedHashString {
		hashStr = hashStr[:ha.lenNormalizedHashString]
	}
	return hashStr, nil
}

func (ha *SHA512HashAlgorithm) HashToHashString(h hash.Hash) (string, error) {
	if reflect.TypeOf(h) != reflect.TypeOf(sha512.New()) {
		return "", fmt.Errorf("wrong hash alghoritm")
	}
	s := h.Sum(nil)
	return ha.sumToHashString(s), nil
}

// sumToHashString takes the hash sum and returns a shortened and prefixed
// hexadecimal string version
func (ha *SHA512HashAlgorithm) sumToHashString(k []byte) string {
	return fmt.Sprintf("%s-%x", ha.hashPrefix, k)[0:ha.lenNormalizedHashString]
}

func (ha *SHA512HashAlgorithm) NewHash() hash.Hash {
	return sha512.New()
}

// SHA256HashAlgorithm is a hash algorithm that uses sha256 sum
type SHA256HashAlgorithm struct {
	hashPrefix       string
	lenHash          int
	lenSum           int
	lenHashString    int
	minlenHashString int
}

func NewSHA256HashAlgorithm() *SHA256HashAlgorithm {
	hashPrefix := "sha256"
	lenHash := sha256.Size  // raw byte size
	lenSum := (lenHash) * 2 // in hex characters
	lenHashString := len(hashPrefix) + 1 + lenSum
	minlenHashString := len(hashPrefix) + 1 + 2 // at least sha256-aa

	return &SHA256HashAlgorithm{
		hashPrefix:       hashPrefix,
		lenHash:          lenHash,
		lenSum:           lenSum,
		lenHashString:    lenHashString,
		minlenHashString: minlenHashString,
	}
}

func (ha *SHA256HashAlgorithm) ValidateHashString(hashStr string) error {
	elems := strings.Split(hashStr, "-")
	if len(elems) != 2 {
		return fmt.Errorf("badly formatted hash string")
	}
	hashPrefix := elems[0]
	if hashPrefix != ha.hashPrefix {
		return fmt.Errorf("wrong hash string prefix %q", hashPrefix)
	}
	if len(hashStr) > ha.lenHashString {
		return fmt.Errorf("hash string too long")
	}
	if len(hashStr) < ha.minlenHashString {
		return fmt.Errorf("hash string too short")
	}
	return nil
}

func (ha *SHA256HashAlgorithm) NormalizeHashString(hashStr string) (string, error) {
	if err := ha.ValidateHashString(hashStr); err != nil {
		return "", err
	}
	return hashStr, nil
}

func (ha *SHA256HashAlgorithm) HashToHashString(h hash.Hash) (string, error) {
	if reflect.TypeOf(h) != reflect.TypeOf(sha256.New()) {
		return "", fmt.Errorf("wrong hash alghoritm")
	}
	s := h.Sum(nil)
	return ha.sumToHashString(s), nil
}

// sumToHashString takes the hash sum and returns a prefixed hexadecimal string
// version
func (ha *SHA256HashAlgorithm) sumToHashString(sum []byte) string {
	return fmt.Sprintf("%s-%x", ha.hashPrefix, sum)[0:ha.lenHashString]
}

func (ha *SHA256HashAlgorithm) NewHash() hash.Hash {
	return sha256.New()
}
