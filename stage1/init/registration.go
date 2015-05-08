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

package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"syscall"
	"time"

	"github.com/coreos/rkt/common"
)

const retryCount = 3

var retryPause = time.Second

var errUnreachable = errors.New(`could not reach the metadata service.
Make sure metadata service is currently running.
For more information on running metadata service,
see https://github.com/coreos/rkt/blob/master/Documentation/metadata-service.md`)

func registerPod(p *Pod, ip net.IP) error {
	uuid := p.UUID.String()

	cmf, err := os.Open(common.PodManifestPath(p.Root))
	if err != nil {
		return fmt.Errorf("failed opening runtime manifest: %v", err)
	}
	defer cmf.Close()

	pth := fmt.Sprintf("/pods/%v?ip=%v", uuid, ip.To4().String())
	if err := httpRequest("PUT", pth, cmf); err != nil {
		return fmt.Errorf("failed to register pod with metadata svc: %v", err)
	}

	for _, app := range p.Manifest.Apps {
		ampath := common.ImageManifestPath(p.Root, app.Image.ID)
		amf, err := os.Open(ampath)
		if err != nil {
			fmt.Errorf("failed reading app manifest %q: %v", ampath, err)
		}
		defer amf.Close()

		if err := registerApp(uuid, app.Name.String(), amf); err != nil {
			fmt.Errorf("failed to register app with metadata svc: %v", err)
		}
	}

	return nil
}

func unregisterPod(p *Pod) error {
	pth := path.Join("/pods", p.UUID.String())
	return httpRequest("DELETE", pth, nil)
}

func registerApp(uuid, app string, r io.Reader) error {
	pth := path.Join("/pods", uuid, app)
	return httpRequest("PUT", pth, r)
}

func httpRequest(method, pth string, body io.Reader) error {
	uri := "http://unixsock" + pth

	t := &http.Transport{
		Dial: func(_, _ string) (net.Conn, error) {
			return net.Dial("unix", common.MetadataServiceRegSock)
		},
	}

	var err error
	for i := 0; i < retryCount; i++ {
		var req *http.Request
		req, err = http.NewRequest(method, uri, body)
		if err != nil {
			return err
		}

		cli := http.Client{Transport: t}

		var resp *http.Response
		resp, err = cli.Do(req)
		switch {
		case err == nil:
			defer resp.Body.Close()

			if resp.StatusCode != 200 {
				return fmt.Errorf("%v %v returned %v", method, pth, resp.StatusCode)
			}

			return nil

		default:
			log.Print(err)
			time.Sleep(retryPause)
		}
	}

	if urlErr, ok := err.(*url.Error); ok {
		if opErr, ok := urlErr.Err.(*net.OpError); ok {
			if opErr.Err == syscall.ENOENT || opErr.Err == syscall.ENOTSOCK {
				return errUnreachable
			}
		}
	}

	return err
}
