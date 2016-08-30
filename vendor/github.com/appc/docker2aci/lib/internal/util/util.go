// Copyright 2015 The appc Authors
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

// Package util defines convenience functions for handling slices and debugging.
//
// Note: this package is an implementation detail and shouldn't be used outside
// of docker2aci.
package util

import (
	"crypto/tls"
	"fmt"
	"net/http"

	"github.com/appc/spec/pkg/acirenderer"
)

// Quote takes a slice of strings and returns another slice with them quoted.
func Quote(l []string) []string {
	var quoted []string

	for _, s := range l {
		quoted = append(quoted, fmt.Sprintf("%q", s))
	}

	return quoted
}

// ReverseImages takes an acirenderer.Images and reverses it.
func ReverseImages(s acirenderer.Images) acirenderer.Images {
	var o acirenderer.Images
	for i := len(s) - 1; i >= 0; i-- {
		o = append(o, s[i])
	}

	return o
}

// In checks whether el is in list.
func In(list []string, el string) bool {
	return IndexOf(list, el) != -1
}

// IndexOf returns the index of el in list, or -1 if it's not found.
func IndexOf(list []string, el string) int {
	for i, x := range list {
		if el == x {
			return i
		}
	}
	return -1
}

// GetTLSClient gets an HTTP client that behaves like the default HTTP
// client, but optionally skips the TLS certificate verification.
func GetTLSClient(skipTLSCheck bool) *http.Client {
	if !skipTLSCheck {
		return http.DefaultClient
	}
	client := *http.DefaultClient
	// Default transport is hidden behind the RoundTripper
	// interface, so we can't easily make a copy of it. If this
	// ever panics, we will have to adapt.
	realTransport := http.DefaultTransport.(*http.Transport)
	tr := *realTransport
	if tr.TLSClientConfig == nil {
		tr.TLSClientConfig = &tls.Config{}
	}
	tr.TLSClientConfig.InsecureSkipVerify = true
	client.Transport = &tr
	return &client
}
