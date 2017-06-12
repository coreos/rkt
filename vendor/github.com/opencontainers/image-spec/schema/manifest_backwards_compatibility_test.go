// Copyright 2016 The Linux Foundation
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

package schema_test

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"github.com/opencontainers/image-spec/schema"
	"github.com/opencontainers/image-spec/specs-go/v1"
)

var compatMap = map[string]string{
	"application/vnd.docker.distribution.manifest.list.v2+json": v1.MediaTypeImageManifestList,
	"application/vnd.docker.distribution.manifest.v2+json":      v1.MediaTypeImageManifest,
	"application/vnd.docker.image.rootfs.diff.tar.gzip":         v1.MediaTypeImageLayer,
	"application/vnd.docker.container.image.v1+json":            v1.MediaTypeImageConfig,
}

// convertFormats converts Docker v2.2 image format JSON documents to OCI
// format by simply replacing instances of the strings found in the compatMap
// found in the input string.
func convertFormats(input string) string {
	out := input
	for k, v := range compatMap {
		out = strings.Replace(out, v, k, -1)
	}
	return out
}

func TestBackwardsCompatibilityManifestList(t *testing.T) {
	for i, tt := range []struct {
		manifest string
		digest   string
		fail     bool
	}{
		{
			digest: "sha256:e588eb8123f2031a41f2e60bc27f30a4388e181e07410aff392f7dc96b585969",
			manifest: `{
   "schemaVersion": 2,
   "mediaType": "application/vnd.docker.distribution.manifest.list.v2+json",
   "manifests": [
      {
         "mediaType": "application/vnd.docker.distribution.manifest.v1+json",
         "size": 2094,
         "digest": "sha256:7820f9a86d4ad15a2c4f0c0e5479298df2aa7c2f6871288e2ef8546f3e7b6783",
         "platform": {
            "architecture": "ppc64le",
            "os": "linux"
         }
      },
      {
         "mediaType": "application/vnd.docker.distribution.manifest.v1+json",
         "size": 1922,
         "digest": "sha256:ae1b0e06e8ade3a11267564a26e750585ba2259c0ecab59ab165ad1af41d1bdd",
         "platform": {
            "architecture": "amd64",
            "os": "linux",
            "features": [
               "sse"
            ]
         }
      },
      {
         "mediaType": "application/vnd.docker.distribution.manifest.v1+json",
         "size": 2084,
         "digest": "sha256:e4c0df75810b953d6717b8f8f28298d73870e8aa2a0d5e77b8391f16fdfbbbe2",
         "platform": {
            "architecture": "s390x",
            "os": "linux"
         }
      },
      {
         "mediaType": "application/vnd.docker.distribution.manifest.v1+json",
         "size": 2084,
         "digest": "sha256:07ebe243465ef4a667b78154ae6c3ea46fdb1582936aac3ac899ea311a701b40",
         "platform": {
            "architecture": "arm",
            "os": "linux",
            "variant": "armv7"
         }
      },
      {
         "mediaType": "application/vnd.docker.distribution.manifest.v1+json",
         "size": 2090,
         "digest": "sha256:fb2fc0707b86dafa9959fe3d29e66af8787aee4d9a23581714be65db4265ad8a",
         "platform": {
            "architecture": "arm64",
            "os": "linux",
            "variant": "armv8"
         }
      }
   ]
}`,
			fail: false,
		},
	} {
		sum := sha256.Sum256([]byte(tt.manifest))
		got := fmt.Sprintf("sha256:%s", hex.EncodeToString(sum[:]))
		if tt.digest != got {
			t.Errorf("test %d: expected digest %s but got %s", i, tt.digest, got)
		}

		manifest := convertFormats(tt.manifest)
		r := strings.NewReader(manifest)
		err := schema.MediaTypeManifestList.Validate(r)

		if got := err != nil; tt.fail != got {
			t.Errorf("test %d: expected validation failure %t but got %t, err %v", i, tt.fail, got, err)
		}
	}
}

func TestBackwardsCompatibilityManifest(t *testing.T) {
	for i, tt := range []struct {
		manifest string
		digest   string
		fail     bool
	}{
		// manifest pulled from docker hub using hash value
		//
		// curl -L -H "Authorization: Bearer ..." -H \
		// "Accept: application/vnd.docker.distribution.manifest.v2+json" \
		// https://registry-1.docker.io/v2/library/docker/manifests/sha256:888206c77cd2811ec47e752ba291e5b7734e3ef137dfd222daadaca39a9f17bc
		{
			digest: "sha256:888206c77cd2811ec47e752ba291e5b7734e3ef137dfd222daadaca39a9f17bc",
			manifest: `{
   "schemaVersion": 2,
   "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
   "config": {
      "mediaType": "application/octet-stream",
      "size": 3210,
      "digest": "sha256:5359a4f250650c20227055957e353e8f8a74152f35fe36f00b6b1f9fc19c8861"
   },
   "layers": [
      {
         "mediaType": "application/vnd.docker.image.rootfs.diff.tar.gzip",
         "size": 2310272,
         "digest": "sha256:fae91920dcd4542f97c9350b3157139a5d901362c2abec284de5ebd1b45b4957"
      },
      {
         "mediaType": "application/vnd.docker.image.rootfs.diff.tar.gzip",
         "size": 913022,
         "digest": "sha256:f384f6ab36adad485192f09379c0b58dc612a3cde82c551e082a7c29a87c95da"
      },
      {
         "mediaType": "application/vnd.docker.image.rootfs.diff.tar.gzip",
         "size": 9861668,
         "digest": "sha256:ed0d2dd5e1a0e5e650a330a864c8a122e9aa91fa6ba9ac6f0bd1882e59df55e7"
      },
      {
         "mediaType": "application/vnd.docker.image.rootfs.diff.tar.gzip",
         "size": 465,
         "digest": "sha256:ec4d00b58417c45f7ddcfde7bcad8c9d62a7d6d5d17cdc1f7d79bcb2e22c1491"
      }
   ]
}`,
			fail: false,
		},
	} {
		sum := sha256.Sum256([]byte(tt.manifest))
		got := fmt.Sprintf("sha256:%s", hex.EncodeToString(sum[:]))
		if tt.digest != got {
			t.Errorf("test %d: expected digest %s but got %s", i, tt.digest, got)
		}

		manifest := convertFormats(tt.manifest)
		r := strings.NewReader(manifest)
		err := schema.MediaTypeManifest.Validate(r)

		if got := err != nil; tt.fail != got {
			t.Errorf("test %d: expected validation failure %t but got %t, err %v", i, tt.fail, got, err)
		}
	}
}

func TestBackwardsCompatibilityConfig(t *testing.T) {
	for i, tt := range []struct {
		manifest string
		digest   string
		fail     bool
	}{
		// manifest pulled from docker hub blob store
		//
		// curl -L -H "Authorization: Bearer ..." -H \
		// -H "Accept: application/vnd.docker.distribution.manifest.v2+json" \
		// https://registry-1.docker.io/v2/library/docker/blobs/sha256:5359a4f250650c20227055957e353e8f8a74152f35fe36f00b6b1f9fc19c8861
		{
			digest:   "sha256:5359a4f250650c20227055957e353e8f8a74152f35fe36f00b6b1f9fc19c8861",
			manifest: `{"architecture":"amd64","config":{"Hostname":"e5e5b3910a57","Domainname":"","User":"","AttachStdin":false,"AttachStdout":false,"AttachStderr":false,"Tty":false,"OpenStdin":false,"StdinOnce":false,"Env":["PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin","DOCKER_BUCKET=get.docker.com","DOCKER_VERSION=1.10.3","DOCKER_SHA256=d0df512afa109006a450f41873634951e19ddabf8c7bd419caeb5a526032d86d"],"Cmd":["sh"],"ArgsEscaped":true,"Image":"sha256:bda352ba7ab5823b7dc74b380c5ad1699edee278a6d2ebbe451129b108778742","Volumes":null,"WorkingDir":"","Entrypoint":["docker-entrypoint.sh"],"OnBuild":[],"Labels":{}},"container":"881be788b4387039b52fa195da9fe26f264385aa497ce650cfdcf3806c2d2021","container_config":{"Hostname":"e5e5b3910a57","Domainname":"","User":"","AttachStdin":false,"AttachStdout":false,"AttachStderr":false,"Tty":false,"OpenStdin":false,"StdinOnce":false,"Env":["PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin","DOCKER_BUCKET=get.docker.com","DOCKER_VERSION=1.10.3","DOCKER_SHA256=d0df512afa109006a450f41873634951e19ddabf8c7bd419caeb5a526032d86d"],"Cmd":["/bin/sh","-c","#(nop) CMD [\"sh\"]"],"ArgsEscaped":true,"Image":"sha256:bda352ba7ab5823b7dc74b380c5ad1699edee278a6d2ebbe451129b108778742","Volumes":null,"WorkingDir":"","Entrypoint":["docker-entrypoint.sh"],"OnBuild":[],"Labels":{}},"created":"2016-06-08T00:52:29.30472774Z","docker_version":"1.10.3","history":[{"created":"2016-06-08T00:48:01.932532048Z","created_by":"/bin/sh -c #(nop) ADD file:bca92e550bd2ce926584aef2032464b6ebf543ce69133b6602c781866165d703 in /"},{"created":"2016-06-08T00:52:10.503417531Z","created_by":"/bin/sh -c apk add --no-cache \t\tca-certificates \t\tcurl \t\topenssl"},{"created":"2016-06-08T00:52:10.700704697Z","created_by":"/bin/sh -c #(nop) ENV DOCKER_BUCKET=get.docker.com","empty_layer":true},{"created":"2016-06-08T00:52:25.746175479Z","created_by":"/bin/sh -c #(nop) ENV DOCKER_VERSION=1.10.3","empty_layer":true},{"created":"2016-06-08T00:52:25.954613633Z","created_by":"/bin/sh -c #(nop) ENV DOCKER_SHA256=d0df512afa109006a450f41873634951e19ddabf8c7bd419caeb5a526032d86d","empty_layer":true},{"created":"2016-06-08T00:52:28.173693898Z","created_by":"/bin/sh -c curl -fSL \"https://${DOCKER_BUCKET}/builds/Linux/x86_64/docker-$DOCKER_VERSION\" -o /usr/local/bin/docker \t\u0026\u0026 echo \"${DOCKER_SHA256}  /usr/local/bin/docker\" | sha256sum -c - \t\u0026\u0026 chmod +x /usr/local/bin/docker"},{"created":"2016-06-08T00:52:28.924486515Z","created_by":"/bin/sh -c #(nop) COPY file:50006c902e7677711aeffe4ab7b7042d649618b96dec760f322a8566dd83ab25 in /usr/local/bin/"},{"created":"2016-06-08T00:52:29.121963047Z","created_by":"/bin/sh -c #(nop) ENTRYPOINT \u0026{[\"docker-entrypoint.sh\"]}","empty_layer":true},{"created":"2016-06-08T00:52:29.30472774Z","created_by":"/bin/sh -c #(nop) CMD [\"sh\"]","empty_layer":true}],"os":"linux","rootfs":{"type":"layers","diff_ids":["sha256:77f08abee8bf9334407f52d104e1891283018450b3c196118ddfe31505126b87","sha256:707d16737060172b977d5f7eaaddfcfaae1008472193d7e8e5a01111a5f8dd5c","sha256:44da042e7b2458ee0b3877f2321cdf4fd45a49b9b51e00492c2ba68055573eff","sha256:1bc2be83dce13b9bac9476c9c1d2ca6e0db3e07b443f7298fc5a1af75b2cb4ef"]}}`,
			fail:     false,
		},
		{
			// fedora:23 from docker hub
			// both Entrypoint and Cmd can be nullable
			digest:   "sha256:a20665eb1fe2912accb3d5dadaed360430df0d1aa46874875886947d61d3d4ee",
			manifest: `{"architecture":"amd64","author":"Patrick Uiterwijk \u003cpatrick@puiterwijk.org\u003e","config":{"Hostname":"8dfe43d80430","Domainname":"","User":"","AttachStdin":false,"AttachStdout":false,"AttachStderr":false,"Tty":false,"OpenStdin":false,"StdinOnce":false,"Env":["PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"],"Cmd":null,"Image":"sha256:6986ae504bbf843512d680cc959484452034965db15f75ee8bdd1b107f61500b","Volumes":null,"WorkingDir":"","Entrypoint":null,"OnBuild":null,"Labels":{}},"container":"6249cd2c4b1d6b1bf05903364cbcb95781508994d6407c1564d494e748ea1b41","container_config":{"Hostname":"8dfe43d80430","Domainname":"","User":"","AttachStdin":false,"AttachStdout":false,"AttachStderr":false,"Tty":false,"OpenStdin":false,"StdinOnce":false,"Env":["PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"],"Cmd":["/bin/sh","-c","#(nop) ADD file:293a6e463aa402bb8f80eb5cfc937f375cedc6843abaeb9eccfe3923bb3fc80b in /"],"Image":"sha256:6986ae504bbf843512d680cc959484452034965db15f75ee8bdd1b107f61500b","Volumes":null,"WorkingDir":"","Entrypoint":null,"OnBuild":null,"Labels":{}},"created":"2016-06-10T18:44:31.784795904Z","docker_version":"1.10.3","history":[{"created":"2016-06-10T18:44:03.360264073Z","author":"Patrick Uiterwijk \u003cpatrick@puiterwijk.org\u003e","created_by":"/bin/sh -c #(nop) MAINTAINER Patrick Uiterwijk \u003cpatrick@puiterwijk.org\u003e","empty_layer":true},{"created":"2016-06-10T18:44:31.784795904Z","author":"Patrick Uiterwijk \u003cpatrick@puiterwijk.org\u003e","created_by":"/bin/sh -c #(nop) ADD file:293a6e463aa402bb8f80eb5cfc937f375cedc6843abaeb9eccfe3923bb3fc80b in /"}],"os":"linux","rootfs":{"type":"layers","diff_ids":["sha256:d43f38155a799dc53d8fbb9f3bc11f51805f4027cd5c3d10b9823201cd5b9400"]}}`,
			fail:     false,
		},
	} {
		sum := sha256.Sum256([]byte(tt.manifest))
		got := fmt.Sprintf("sha256:%s", hex.EncodeToString(sum[:]))
		if tt.digest != got {
			t.Errorf("test %d: expected digest %s but got %s", i, tt.digest, got)
		}

		manifest := convertFormats(tt.manifest)
		r := strings.NewReader(manifest)
		err := schema.MediaTypeImageConfig.Validate(r)

		if got := err != nil; tt.fail != got {
			t.Errorf("test %d: expected validation failure %t but got %t, err %v", i, tt.fail, got, err)
		}
	}
}
