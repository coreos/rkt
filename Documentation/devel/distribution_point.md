
A distribution point represents the method to fetch an image starting from an input string. It doesn't mandates the image types. Usually a distribution point can only provide one image type but some can provide different image types. For example:

* Some docker registries also provides OCI images.
* Rkt can internally implement the docker distribution to natively fetch a docker/OCI image from a registry or an ACI converted by docker2aci.

Before this concept was introduced, rkt used the ImageType concept, the ImageType was mapped to the input image string format and internally covered multiple concepts like distribution, transport and image type (hidden since all are now appc ACIs).

The distribution point concept is used as the primary information in the different layers of rkt (fetching but also for references in a CAS/ref store).

## Distribution Points types

Distribution points can be logically considered of two types:

* Indirect distribution points
* Direct distribution points

Direct distribution points already provide the final information needed to fetch the image while Indirect distribution points will do some indirect steps (e.g. discovery) to get the final image location.

It can happen, as explained below, that an Indirect distribution point will resolve to a Direct distribution point.

## Distribution Points format
A distribution point is represented as an URI with the uri scheme as "cimd" and the remaining parts (URI opaque data and query/fragments parts) as the distribution point data. See [rfc3986](https://tools.ietf.org/html/rfc3986)). Distribution points clearly maps to a resource name, instead they won't fit inside a resource locator (URL). We'll use the term URIs instead of URNs because it's the suggested name from the rfc (and URNs are defined, by rfc2141, to have the `urn` scheme).

Every distribution starts with a common part: `cimd:DISTTYPE:v=uint32(VERSION):` where `cimd` is the container image distribution scheme, DISTTYPE is the distribution type, v=uint32(VERSION) is the distribution type format version.

### Current rkt Distribution Points

In rkt there are currently three kind of distribution points: `Appc`, `ACIArchive` and `Docker`. This is their formalization:

**Appc**
This is an Indirect distribution point.

Appc defines a distribution point using appc image discovery

The format is: `cimd:appc:v=0:name?label01=....&label02=....`
The distribution type is "appc"
the labels values must be Query escaped
Example: `cimd:appc:v=0:coreos.com/etcd?version=v3.0.3&os=linux&arch=amd64`

**ACIArchive**
This is a Direct distribution point since it directly define the final image location.

ACIArchive defines a distribution point using an archive file

The format is: `cimd:aci-archive:v=0:ArchiveURL?query...`
The distribution type is "aci-archive"
ArchiveURL must be query escaped

Examples:
`cimd:aci-archive:v=0:file%3A%2F%2Fabsolute%2Fpath%2Fto%2Ffile`
`cimd:aci-archive:v=0:https%3A%2F%2Fexample.com%2Fapp.aci`

**Docker**
This is an Indirect distribution point.

Docker defines a distribution point using a docker registry

The format is:
`cimd:docker:v=0:[REGISTRY_HOST[:REGISTRY_PORT]/]NAME[:TAG|@DIGEST]`
Removing the common distribution point part, the format is the same as the docker image string format (man docker-pull).

Examples:
`cimd:docker:v=0:busybox`
`cimd:docker:v=0:busybox:latest`
`cimd:docker:v=0:registry-1.docker.io/library/busybox@sha256:a59906e33509d14c036c8678d687bd4eec81ed7c4b8ce907b888c607f6a1e0e6`

### Future distributions points

**OCI Image distribution(s)**
This is an Indirect distribution point.

Currently oci images can be retrieved using a docker registry but in future the oci image spec will define one or more own kinds of distribution starting from an image name (with additional tags/labels).

**OCI Image layout**
This is a Direct distribution point.

It can fetch an image starting from a [OCI image layout](https://github.com/opencontainers/image-spec/blob/master/image-layout.md) format. The location can point to a single file archive, to a local/remote directory based layout or other kind of locations.

Probably it will be the final distribution used by the above OCI image distributionis (like ACIArchive is the final distribution point for the Appc distribution point).

`cimd:oci-image-layout:v=0:file%3A%2F%2Fabsolute%2Fpath%2Fto%2Ffile?ref=refname`
`cimd:oci-image-layout:v=0:https%3A%2F%2Fdir%2F?ref=refname`

Since the OCI image layout can provide multiple images selectable by a ref there's the need to specify which ref to use in the archive distribution URI (see the above ref query parameter). Since distribution just covers one image it's not possible to import all the refs with just a distribution URI.

TODO(sgotti). Define if oci-image-layout should internally handle both archive and directory based layouts or use two different distributions or a query parameter the explicitly define the layout (to avoid guessing if the url points to a single file or to a directory).

**Note** Considering [this OCI image spec README section](https://github.com/opencontainers/image-spec#running-an-oci-image), probably the final distribution format will be similar to the Appc distribution. So there's a need to distinguish their User Friendly string (prepending an appc: or oci: ?).

To sum it up:

| Distribution Point | Type     | URI Format                                                                | Final Distribution Point |
|--------------------|----------|---------------------------------------------------------------------------|--------------------------|
| Appc               | Direct   | `cimd:appc:v=0:name?label01=....&label02=...`                             | ACIArchive               |
| Docker             | Direct   | `cimd:docker:v=0:[REGISTRY_HOST[:REGISTRY_PORT]/]NAME[:TAG&#124;@DIGEST]` | <none>                   |
| ACIArchive         | Indirect | `cimd:aci-archive:v=0:ArchiveURL?query...`                                |                          |
| OCI                | Direct   | `cimd:oci:v=0:TODO`                                                       | OCIImageLayout           |
| OCIImageLayout     | Indirect | `cimd:oci-image-layout:v=0:URL?ref=...`                                   |                          |

## User friendly distribution strings

Since the distribution URI can be complex there's a need to help the user to request an image via some user friendly string. rkt already has these kind of available input image strings (now mapped to an AppImageType):

* appc discovery string: example.com/app01:v1.0.0,label01=value01,... or example.com/app01,version=v1.0.0,label01=value01,... etc...
* file path: absolute /full/path/to/file or relative
The above two may overlap so some heuristic is needed to distinguish them (removing this heuristic will break backward cli compatibility).
* file URL: file:///full/path/to/file
* http(s) URL: http(s)://host:port/path
* docker URL: this is a strange URL since it the schemeful (docker://) version of the docker image string

So, also the maintain backward compatibility, these image string will be converted to a distribution URI:

| Current ImageType                      | Distribution Point URI                                                          |
|----------------------------------------|---------------------------------------------------------------------------|
| appc string                            | `cimd:appc:v=0:name?label01=....&label02=...`                             |
| file path                              | `cimd:aci-archive:v=0:ArchiveURL`                                          |
| file URL                               | `cimd:aci-archive:v=0:ArchiveURL`                                          |
| https URL                              | `cimd:aci-archive:v=0:ArchiveURL`                                          |
| docker URI/URL (docker: and docker://) | `cimd:docker:v=0:[REGISTRY_HOST[:REGISTRY_PORT]/]NAME[:TAG&#124;@DIGEST]` |

The above table also adds docker URI (docker:) as a user friendly string and its clearer than the URL version (docker://)

The parsing and generation of user friendly string is done outside the distribution package (to let distribution pkg users implement their own user friendly strings).

rkt has two functions to:
* parse a user friendly string to a distribution URI.
* generate a user friendly string from a distribution URI. This is useful for example when showing the refs from a refs store (so they'll be easily understandable and if copy/pasted they'll continue to work).

A user can provide as an input image both the "user friendly" strings or directly a distribution URI.

## Comparing Distribution Points URIs

A Distribution Point implementation will also provide:

* a function to compare if two Distribution Point URIs are the same (e.g. ordering the query parameters).

## Fetching logic with Distribution Points

A Distribution Point will be the base for a future refactor of the fetchers logic (see #2964)

This will also creates a better separation between the distribution points and the transport layers.

For example there may exist multiple transport plugins (file, http, s3, bittorrent etc...) to be called by an ACIArchive distribution point.

