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

package stage0

import (
	"crypto/rand"
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

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/schema"
	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/schema/types"

	"github.com/coreos/rkt/common"
)

const retryCount = 3

var retryPause = time.Second

var errUnreachable = errors.New(`could not reach the metadata service.
Make sure metadata service is currently running.
For more information on running metadata service,
see https://github.com/coreos/rkt/blob/master/Documentation/metadata-service.md`)

// registerPod registers pod with metadata service.
// Returns authentication token to be passed in the URL
func registerPod(root string, uuid *types.UUID, apps schema.AppList) (token string, rerr error) {
	u := uuid.String()

	var err error
	token, rerr = generateMDSToken()
	if err != nil {
		rerr = fmt.Errorf("failed to generate MDS token: %v", err)
		return
	}

	pmfPath := common.PodManifestPath(root)
	pmf, err := os.Open(pmfPath)
	if err != nil {
		rerr = fmt.Errorf("failed to open runtime manifest (%v): %v", pmfPath, err)
		return
	}

	pth := fmt.Sprintf("/pods/%v?token=%v", u, token)
	err = httpRequest("PUT", pth, pmf)
	pmf.Close()
	if err != nil {
		rerr = fmt.Errorf("failed to register pod with metadata svc: %v", err)
		return
	}

	defer func() {
		if rerr != nil {
			unregisterPod(uuid)
		}
	}()

	for _, app := range apps {
		ampath := common.ImageManifestPath(root, app.Image.ID)
		amf, err := os.Open(ampath)
		if err != nil {
			rerr = fmt.Errorf("failed reading app manifest %q: %v", ampath, err)
			return
		}

		err = registerApp(u, app.Name.String(), amf)
		amf.Close()
		if err != nil {
			rerr = fmt.Errorf("failed to register app with metadata svc: %v", err)
			return
		}
	}

	return
}

// unregisterPod unregisters pod with the metadata service.
func unregisterPod(uuid *types.UUID) error {
	pth := path.Join("/pods", uuid.String())
	return httpRequest("DELETE", pth, nil)
}

func generateMDSToken() (string, error) {
	bytes := make([]byte, 16)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", bytes), nil
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
