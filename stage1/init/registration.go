// Copyright 2014 CoreOS, Inc.
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
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path"

	"github.com/coreos/rocket/common"
)

func registerContainer(c *Container, ip net.IP) error {
	cmf, err := os.Open(common.ContainerManifestPath(c.Root))
	if err != nil {
		return fmt.Errorf("failed opening runtime manifest: %v", err)
	}
	defer cmf.Close()

	pth := fmt.Sprintf("/containers/?ip=%v", ip.To4().String())
	if err := httpRequest("POST", pth, cmf); err != nil {
		return fmt.Errorf("failed to register container with metadata svc: %v", err)
	}

	uid := c.Manifest.UUID.String()
	for _, app := range c.Manifest.Apps {
		ampath := common.ImageManifestPath(c.Root, app.Image.ID)
		amf, err := os.Open(ampath)
		if err != nil {
			fmt.Errorf("failed reading app manifest %q: %v", ampath, err)
		}
		defer amf.Close()

		if err := registerApp(uid, app.Name.String(), amf); err != nil {
			fmt.Errorf("failed to register app with metadata svc: %v", err)
		}
	}

	return nil
}

func unregisterContainer(c *Container) error {
	pth := path.Join("/containers", c.Manifest.UUID.String())
	return httpRequest("DELETE", pth, nil)
}

func registerApp(uuid, app string, r io.Reader) error {
	pth := path.Join("/containers", uuid, app)
	return httpRequest("PUT", pth, r)
}

func httpRequest(method, pth string, body io.Reader) error {
	uri := common.MetadataSvcPrivateURL() + pth
	req, err := http.NewRequest(method, uri, body)
	if err != nil {
		return err
	}

	cli := http.Client{}

	resp, err := cli.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("%v %v returned %v", method, pth, resp.StatusCode)
	}

	return nil
}
