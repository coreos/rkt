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

package docker

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"runtime"
	"strings"

	"github.com/appc/docker2aci/lib/internal/types"

	"github.com/docker/distribution/reference"
)

const (
	dockercfgFileName    = "config.json"
	dockercfgFileNameOld = ".dockercfg"
	defaultIndexURL      = "registry-1.docker.io"
	defaultIndexURLAuth  = "https://index.docker.io/v1/"
	defaultTag           = "latest"
	defaultRepoPrefix    = "library/"
)

// SplitReposName breaks a repo name into an index name and remote name.
func SplitReposName(name string) (indexName, remoteName string) {
	i := strings.IndexRune(name, '/')
	if i == -1 || (!strings.ContainsAny(name[:i], ".:") && name[:i] != "localhost") {
		indexName, remoteName = defaultIndexURL, name
	} else {
		indexName, remoteName = name[:i], name[i+1:]
	}
	if indexName == defaultIndexURL && !strings.ContainsRune(remoteName, '/') {
		remoteName = defaultRepoPrefix + remoteName
	}
	return
}

// Get a repos name and returns the right reposName + tag
// The tag can be confusing because of a port in a repository name.
//     Ex: localhost.localdomain:5000/samalba/hipache:latest
func parseRepositoryTag(repos string) (string, string) {
	n := strings.LastIndex(repos, ":")
	if n < 0 {
		return repos, ""
	}
	if tag := repos[n+1:]; !strings.Contains(tag, "/") {
		return repos[:n], tag
	}
	return repos, ""
}

func decodeDockerAuth(s string) (string, string, error) {
	decoded, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return "", "", err
	}
	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid auth configuration file")
	}
	user := parts[0]
	password := strings.Trim(parts[1], "\x00")
	return user, password, nil
}

func getHomeDir() string {
	if runtime.GOOS == "windows" {
		return os.Getenv("USERPROFILE")
	}
	return os.Getenv("HOME")
}

// GetDockercfgAuth reads a ~/.dockercfg file and returns the username and password
// of the given docker index server.
func GetAuthInfo(indexServer string) (string, string, error) {
	// official docker registry
	if indexServer == defaultIndexURL {
		indexServer = defaultIndexURLAuth
	}
	dockerCfgPath := path.Join(getHomeDir(), ".docker", dockercfgFileName)
	if _, err := os.Stat(dockerCfgPath); err == nil {
		j, err := ioutil.ReadFile(dockerCfgPath)
		if err != nil {
			return "", "", err
		}
		var dockerAuth types.DockerConfigFile
		if err := json.Unmarshal(j, &dockerAuth); err != nil {
			return "", "", err
		}
		// try the normal case
		if c, ok := dockerAuth.AuthConfigs[indexServer]; ok {
			return decodeDockerAuth(c.Auth)
		}
	} else if os.IsNotExist(err) {
		oldDockerCfgPath := path.Join(getHomeDir(), dockercfgFileNameOld)
		if _, err := os.Stat(oldDockerCfgPath); err != nil {
			return "", "", nil //missing file is not an error
		}
		j, err := ioutil.ReadFile(oldDockerCfgPath)
		if err != nil {
			return "", "", err
		}
		var dockerAuthOld map[string]types.DockerAuthConfigOld
		if err := json.Unmarshal(j, &dockerAuthOld); err != nil {
			return "", "", err
		}
		if c, ok := dockerAuthOld[indexServer]; ok {
			return decodeDockerAuth(c.Auth)
		}
	} else {
		// if file is there but we can't stat it for any reason other
		// than it doesn't exist then stop
		return "", "", fmt.Errorf("%s - %v", dockerCfgPath, err)
	}
	return "", "", nil
}

// ParseDockerURL takes a Docker URL and returns a ParsedDockerURL with its
// index URL, image name, and tag.
func ParseDockerURL(arg string) (*types.ParsedDockerURL, error) {
	r, err := reference.ParseNamed(arg)
	if err != nil {
		return nil, err
	}

	tag := defaultTag
	var digest string
	switch x := r.(type) {
	case reference.Canonical:
		digest = x.Digest().String()
	case reference.NamedTagged:
		tag = x.Tag()
	}

	indexURL, remoteName := SplitReposName(r.Name())

	p := &types.ParsedDockerURL{
		IndexURL:  indexURL,
		ImageName: remoteName,
		Tag:       tag,
		Digest:    digest,
	}
	return p, nil
}
