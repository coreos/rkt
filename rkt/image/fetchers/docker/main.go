package main

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	docker2aci "github.com/appc/docker2aci/lib"
	d2acommon "github.com/appc/docker2aci/lib/common"
	"github.com/coreos/rkt/rkt/image/fetchers"
)

func errAndExit(format string, a ...interface{}) {
	out := fmt.Sprintf(format, a...)
	fmt.Fprintln(os.Stderr, strings.TrimSuffix(out, "\n"))
	os.Exit(1)
}

func main() {
	config, err := fetchers.ConfigFromStdin()
	if err != nil {
		errAndExit("error reading config: %v", err)
	}

	if config.Version != 1 {
		errAndExit("unsupported plugin schema version")
	}

	if !config.InsecureOpts.SkipImageCheck {
		errAndExit("signature verification for docker images is not supported")
	}

	err = fetch(config)
	if err != nil {
		errAndExit("error fetching docker image: %v", err)
	}
}

func fetch(config *fetchers.Config) error {
	tmpDir, err := ioutil.TempDir("", "rkt-fetcher-docker")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	u, err := d2acommon.ParseDockerURL(config.Name)
	if err != nil {
		return err
	}

	d2aconfig := docker2aci.RemoteConfig{
		Insecure: d2acommon.InsecureConfig{
			SkipVerify: config.InsecureOpts.SkipImageCheck,
			AllowHTTP:  config.InsecureOpts.AllowHTTP,
		},
		CommonConfig: docker2aci.CommonConfig{
			Squash:      true,
			OutputDir:   tmpDir,
			TmpDir:      tmpDir,
			Compression: d2acommon.NoCompression,
		},
	}
	headers, headersExist := config.Headers[u.IndexURL]
	if headersExist {
		authHeaders, hasAuthHeaders = headers["Authorization"]
		if hasAuthHeaders {
			encodedAuth := ""
			n, err := fmt.Sscanf(authHeaders[0], "Bearer %s", &encodedAuth)
			if err != nil {
				return fmt.Errorf("couldn't parse auth header: %s", err)
			}
			if n != 1 {
				return fmt.Errorf("couldn't parse auth header")
			}
			decodedAuth := base64.StdEncoding.DecodeString(encodedAuth)
			user := ""
			password := ""
			n, err = fmt.Sscanf(decodedAuth, "%s:%s", &user, &password)
			if err != nil {
				return fmt.Errorf("couldn't parse auth header contents: %s", err)
			}
			if n != 2 {
				return fmt.Errorf("couldn't parse auth header contents")
			}
			d2aconfig.Username = user
			d2aconfig.Password = password
		}
	}
	acis, err := docker2aci.ConvertRemoteRepo(config.Name, d2aconfig)
	if err != nil {
		return fmt.Errorf("error converting docker image to ACI: %v", err)
	}

	aciFile, err := os.Open(acis[0])
	if err != nil {
		return fmt.Errorf("error opening squashed ACI file: %v", err)
	}

	err = os.Rename(aciFile.Name(), config.OutputACIPath)
	if err != nil {
		return fmt.Errorf("%v", err)
	}

	return fetchers.ResultToStdout(&fetchers.Result{
		Latest: u.Tag == "latest",
	})
}
