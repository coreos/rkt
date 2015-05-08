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

package aci

import (
	"crypto/tls"
	"fmt"
	"net/http"
)

func StartServer(auth Type) (*Server, error) {
	return NewServer(auth, 10)
}

func StopServer(host string) (*http.Response, error) {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: transport}
	res, err := client.Post(host, "whatever", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to send post to %q: %v", host, err)
	}
	return res, nil
}
