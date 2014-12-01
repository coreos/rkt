package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/coreos/rocket/metadata"
	rktpath "github.com/coreos/rocket/path"
)

func registerContainer(c *Container, ip string) error {
	cmf, err := os.Open(rktpath.ContainerManifestPath(c.Root))
	if err != nil {
		return fmt.Errorf("failed opening runtime manifest: %v\n", err)
	}
	defer cmf.Close()

	path := fmt.Sprintf("/containers/?ip=%v", ip)
	if err := httpRequest("POST", path, cmf); err != nil {
		return fmt.Errorf("failed to register container with metadata svc: %v\n", err)
	}

	uid := c.Manifest.UUID.String()
	for _, app := range c.Manifest.Apps {
		ampath := rktpath.AppManifestPath(c.Root, app.ImageID)
		amf, err := os.Open(ampath)
		if err != nil {
			fmt.Errorf("failed reading app manifest %q: %v\n", ampath, err)
		}
		defer amf.Close()

		if err := registerApp(uid, app.Name.String(), amf); err != nil {
			fmt.Errorf("failed to register app with metadata svc: %v\n", err)
		}
	}

	return nil
}

func unregisterContainer(c *Container) error {
	path := fmt.Sprintf("/containers/%v", c.Manifest.UUID.String())
	return httpRequest("DELETE", path, nil)
}

func registerApp(uuid, app string, r io.Reader) error {
	path := filepath.Join("/containers", uuid, app)
	return httpRequest("PUT", path, r)
}

func httpRequest(method, path string, body io.Reader) error {
	uri := metadata.MetadataSvcPrvURL() + path
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
		return fmt.Errorf("%v %v returned %v", method, path, resp.StatusCode)
	}

	return nil
}
