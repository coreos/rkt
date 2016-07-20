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

import "testing"

func TestSHA512NormalizeHashString(t *testing.T) {
	// This also checks ValidateHashString since it's called but NormalizeHashString

	tests := []struct {
		input          string
		expectedErr    string
		expectedOutput string
	}{
		{
			input:       "sha512-a-a",
			expectedErr: "badly formatted hash string",
		},
		{
			input:       "",
			expectedErr: "badly formatted hash string",
		},
		{
			input:       "00000000000000000000000000000000000000000000000000",
			expectedErr: "badly formatted hash string",
		},
		{
			input:       "sha256-00000000000000000000000000000000000000000000000000",
			expectedErr: `wrong hash string prefix "sha256"`,
		},
		{
			input:       "sha512-1",
			expectedErr: "hash string too short",
		},
		{
			input:       "sha512-67147019a5b56f5e2ee01e989a8aa4787f56b8445960be2d8678391cf111009bc0780f31001fd181a2b61507547aee4caa44cda4b8bdb238d0e4ba830069ed2c1",
			expectedErr: "hash string too long",
		},
		{
			input:          "sha512-67147019a5b56f5e2ee01e989a8aa4787f56b8445960be2d8678391cf111009bc0780f31001fd181a2b61507547aee4caa44cda4b8bdb238d0e4ba830069ed2c",
			expectedOutput: "sha512-67147019a5b56f5e2ee01e989a8aa4787f56b8445960be2d8678391cf111009b",
		},
		{
			input:          "sha512-67147019a5b56f5e2ee01e989a8aa4787f56b8445960be2d8678391cf111009b",
			expectedOutput: "sha512-67147019a5b56f5e2ee01e989a8aa4787f56b8445960be2d8678391cf111009b",
		},
		{
			input:          "sha512-67",
			expectedOutput: "sha512-67",
		},
	}
	for _, tt := range tests {
		ha := NewSHA512HashAlgorithm()
		out, err := ha.NormalizeHashString(tt.input)
		if err != nil {
			if tt.expectedErr != "" {
				if err.Error() != tt.expectedErr {
					t.Fatalf("got err: %v, expecting: %v", err, tt.expectedErr)
				}
			} else {
				t.Fatalf("unexpected error: %v", err)
			}
		} else {
			if tt.expectedErr != "" {
				t.Fatalf("got nil error, expecting: %v", tt.expectedErr)
			}
			if out != tt.expectedOutput {
				t.Fatalf("got normalized hash string: %q, expected: %q", out, tt.expectedOutput)
			}
		}
	}
}

func TestSHA512HashToHashString(t *testing.T) {
	tests := []struct {
		input          string
		expectedOutput string
	}{
		{
			input:          "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			expectedOutput: "sha512-ad9e7ae1f68786c33ca713d4632b29ebcc9c9c040fc176ead8acb395a14c0832",
		},
	}
	for _, tt := range tests {
		ha := NewSHA512HashAlgorithm()
		h := ha.NewHash()
		_, err := h.Write([]byte(tt.input))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		out, err := ha.HashToHashString(h)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out != tt.expectedOutput {
			t.Fatalf("got hash string: %q, expected: %q", out, tt.expectedOutput)
		}
	}
}

func TestSHA256NormalizeHashString(t *testing.T) {
	// This also checks ValidateHashString since it's called but NormalizeHashString

	tests := []struct {
		input          string
		expectedErr    string
		expectedOutput string
	}{
		{
			input:       "sha256-a-a",
			expectedErr: "badly formatted hash string",
		},
		{
			input:       "",
			expectedErr: "badly formatted hash string",
		},
		{
			input:       "00000000000000000000000000000000000000000000000000",
			expectedErr: "badly formatted hash string",
		},
		{
			input:       "sha512-00000000000000000000000000000000000000000000000000",
			expectedErr: `wrong hash string prefix "sha512"`,
		},
		{
			input:       "sha256-1",
			expectedErr: "hash string too short",
		},
		{
			input:       "sha256-67147019a5b56f5e2ee01e989a8aa4787f56b8445960be2d8678391cf111009bc",
			expectedErr: "hash string too long",
		},
		{
			input:          "sha256-67147019a5b56f5e2ee01e989a8aa4787f56b8445960be2d8678391cf111009b",
			expectedOutput: "sha256-67147019a5b56f5e2ee01e989a8aa4787f56b8445960be2d8678391cf111009b",
		},
		{
			input:          "sha256-67",
			expectedOutput: "sha256-67",
		},
	}
	for _, tt := range tests {
		ha := NewSHA256HashAlgorithm()
		out, err := ha.NormalizeHashString(tt.input)
		if err != nil {
			if tt.expectedErr != "" {
				if err.Error() != tt.expectedErr {
					t.Fatalf("got err: %v, expecting: %v", err, tt.expectedErr)
				}
			} else {
				t.Fatalf("unexpected error: %v", err)
			}
		} else {
			if tt.expectedErr != "" {
				t.Fatalf("got nil error, expecting: %v", tt.expectedErr)
			}
			if out != tt.expectedOutput {
				t.Fatalf("got normalized hash string: %q, expected: %q", out, tt.expectedOutput)
			}
		}
	}
}

func TestSHA256HashToHashString(t *testing.T) {
	tests := []struct {
		input          string
		expectedOutput string
	}{
		{
			input:          "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			expectedOutput: "sha256-61c60b487d1a921e0bcc9bf853dda0fb159b30bf57b2e2d2c753b00be15b5a09",
		},
	}
	for _, tt := range tests {
		ha := NewSHA256HashAlgorithm()
		h := ha.NewHash()
		_, err := h.Write([]byte(tt.input))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		out, err := ha.HashToHashString(h)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if out != tt.expectedOutput {
			t.Fatalf("got hash string: %q, expected: %q", out, tt.expectedOutput)
		}
	}
}
