## v0.16.0

This release adds a manifest hash annotation on converted images and introduces some API changes to allow for more granular control on registries and media types.

 - Annotate manifest hash ([#237](https://github.com/appc/docker2aci/pull/237)).
 - Allow selective disabling of registries and media types ([#239](https://github.com/appc/docker2aci/pull/239)).
 - Update appc/spec to 0.8.10 ([#242](https://github.com/appc/docker2aci/pull/242)).

## v0.15.0

This release improves translation of arch labels and image name annotations. It also changes the default output image filename.

 - Translate "os" and "arch" labels of image manifest ([#234](https://github.com/appc/docker2aci/pull/234)).
 - Minor style changes ([#230](https://github.com/appc/docker2aci/pull/230)).
 - Bump appc/spec library version to 0.8.9 ([#233](https://github.com/appc/docker2aci/pull/233)).
 - Image from file improvements; guesses at "originalname" and fixes for "--image"([#229](https://github.com/appc/docker2aci/pull/229)).

## v0.14.0

This release adds compatibility for OCI v1.0.0-rc2 types, introduces supports for converting image labels, and fixes some issues related to automatic fallback to registry API v1.

 - log: introduce Logger interface ([#218](https://github.com/appc/docker2aci/pull/218))
 - lib/internal: set UserLabels to be Docker image labels ([#223](https://github.com/appc/docker2aci/pull/223)).
 - fetch: annotate originally requested name ([#224](https://github.com/appc/docker2aci/pull/224)).
 - types: update OCI image-spec to rc2 ([#226](https://github.com/appc/docker2aci/pull/226)).
 - lib/internal: fix v2 registry check URL ([#220](https://github.com/appc/docker2aci/pull/220))
 - lib/internal: allow auto fallback from v2 API to v1 ([#222](https://github.com/appc/docker2aci/pull/222)).

## v0.13.0

This release adds support for converting local OCI bundles and fixes two security issues (CVE-2016-7569 and CVE-2016-8579). It also includes fixes for several image fetching and conversion bugs.

 - docker2aci: add support for converting OCI tarfiles ([#200](https://github.com/appc/docker2aci/pull/200)).
 - docker2aci: additional validation on malformed images ([#204](https://github.com/appc/docker2aci/pull/204)). Fixes (CVE-2016-7569 and CVE-2016-8579).
 - lib: Use the new media types for oci ([#213](https://github.com/appc/docker2aci/pull/213)).
 - backend/repository: assume no v2 on unexpected status ([#214](https://github.com/appc/docker2aci/pull/214)).
 - lib/internal: do not compare tag when pulling by digest ([#207](https://github.com/appc/docker2aci/pull/207)).
 - lib/internal: re-use uid value when gid is missing ([#206](https://github.com/appc/docker2aci/pull/206)).
 - lib/internal: add entrypoint/cmd annotations to v21 images ([#199](https://github.com/appc/docker2aci/pull/199)).

## v0.12.3

This is another bugfix release.

- lib/repository2: get the correct layer index ([#188](https://github.com/appc/docker2aci/pull/188)). This fixes layer ordering for the Docker API v2.1.
- lib/repository2: fix manifest v2.2 layer ordering ([#190](https://github.com/appc/docker2aci/pull/190)). This fixes layer ordering for the Docker API v2.2.

## v0.12.2

This is a bugfix release.

- lib/repository2: populate reverseLayers correctly ([#185](https://github.com/appc/docker2aci/pull/185)). It caused converted Image Manifests to have the wrong fields. Add a test to make sure this won't go unnoticed again.
- tests: remove redundant code and simplify ([#186](https://github.com/appc/docker2aci/pull/186)).

## v0.12.1

This release fixes a couple of bugs, adds image fetching tests, and replaces godep with glide for vendoring.

- Replace Godeps with glide ([#174](https://github.com/appc/docker2aci/pull/174)).
- Avoid O(N) and fix defer reader close ([#180](https://github.com/appc/docker2aci/pull/180)).
- Add golang tests to lib/test to test image fetching ([#181](https://github.com/appc/docker2aci/pull/181)).

## v0.12.0

v0.12.0 introduces support for the Docker v2.2 image format and OCI image format. It also fixes a bug that prevented pulling by digest to work.

- backend/repository2: don't ignore when there's an image digest ([#171](https://github.com/appc/docker2aci/pull/171)).
- lib/repository2: add support for docker v2.2 and OCI ([#176](https://github.com/appc/docker2aci/pull/176)).

## v0.11.1

v0.11.1 is a bugfix release.

- Fix parallel pull synchronisation ([#167](https://github.com/appc/docker2aci/pull/167), [#168](https://github.com/appc/docker2aci/pull/168)).

## v0.11.0

This release splits the `--insecure` flag in two, `--insecure-skip-verify` to skip TLS verification, and `--insecure-allow-http` to allow unencrypted connections when fetching images. It also includes a couple of bugfixes.

- Add missing message to channel on successful layer download ([#161](https://github.com/appc/docker2aci/pull/161)).
- Fix a panic when a layer being fetched encounters an error ([#162](https://github.com/appc/docker2aci/pull/162)).
- Split `--insecure` flag in two ([#163](https://github.com/appc/docker2aci/pull/163)).

## v0.10.0

This release includes two major performance optimizations: parallel layer pull and parallel ACI compression.

- Pull layers in parallel ([#158](https://github.com/appc/docker2aci/pull/158)).
- Use a parallel compression library ([#157](https://github.com/appc/docker2aci/pull/157)).
- Fix auth token parsing to handle services with spaces in their names ([#150](https://github.com/appc/docker2aci/pull/150)).

## v0.9.3

v0.9.3 is a minor bug fix release.

- Use the default transport when doing HTTP requests ([#147](https://github.com/appc/docker2aci/pull/147)). We were using an empty transport which didn't pass on the proxy configuration.

## v0.9.2

v0.9.2 is a minor release with a bug fix and a cleanup over the previous one.

- Use upstream docker functions to parse docker URLs and parse digest ([#140](https://github.com/appc/docker2aci/pull/140)).
- Change docker entrypoint/cmd annotations to json ([#142](https://github.com/appc/docker2aci/pull/142)).

## v0.9.1

v0.9.1 is mainly a bugfix and cleanup release.

- Remove redundant dependency fetching, we're vendoring them now ([#134](https://github.com/appc/docker2aci/pull/134)).
- Export ParseDockerURL which is used by rkt ([#135](https://github.com/appc/docker2aci/pull/135)).
- Export annotations so people can use them outside docker2aci ([#135](https://github.com/appc/docker2aci/pull/135)).
- Refactor the library so internal functions are in the "internal" package ([#135](https://github.com/appc/docker2aci/pull/135)).
- Document release process and add a bump-version script ([#137](https://github.com/appc/docker2aci/pull/137)).

## v0.9.0

v0.9.0 is the initial release of docker2aci.

docker2aci converts to ACI Docker images from a remote repository or from a local file generated with "docker save".

It supports v1 and v2 Docker registries, compression, and layer squashing.
