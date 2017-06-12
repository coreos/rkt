# OCI Image Manifest List Specification

The manifest list is a higher-level manifest which points to specific [image manifests](manifest.md) for one or more platforms.
While the use of a manifest list is OPTIONAL for image providers, image consumers SHOULD be prepared to process them.
A client will distinguish a manifest list from an image manifest based on the Content-Type returned in the HTTP response.

This section defines the `application/vnd.oci.image.manifest.list.v1+json` [media type](media-types.md).

## *Manifest List* Property Descriptions

- **`schemaVersion`** *int*

  This REQUIRED property specifies the image manifest schema version.
  For this version of the specification, this MUST be `2` to ensure backward compatibility with older versions of Docker. The value of this field will not change. This field MAY be removed in a future version of the specification.

- **`mediaType`** *string*

  This REQUIRED property contains the media type of the manifest list.
  For this version of the specification, this MUST be set to `application/vnd.oci.image.manifest.list.v1+json`.
  For the media type(s) that this is compatible with see the [matrix](media-types.md#compatibility-matrix).

- **`manifests`** *array*

  This REQUIRED property contains a list of manifests for specific platforms.
  While the property MUST be present, the size of the array MAY be zero.

  Each object in the manifest is a [descriptor](descriptor.md) with the following additional properties and restrictions:

  - **`mediaType`** *string*

    This [descriptor property](descriptor.md#properties) has additional restrictions for `manifests`.
    Implementations MUST support at least the following media types:

    - [`application/vnd.oci.image.manifest.v1+json`](manifest.md)

    Manifest lists concerned with portability SHOULD use one of the above media types.

  - **`platform`** *object*

    This REQUIRED property describes the platform which the image in the manifest runs on.
    A full list of valid operating system and architecture values are listed in the [Go language documentation for `$GOOS` and `$GOARCH`](https://golang.org/doc/install/source#environment)

    - **`architecture`** *string*

        This REQUIRED property specified the CPU architecture, for example `amd64` or `ppc64le`.

    - **`os`** *string*

        This REQUIRED property specifies the operating system, for example `linux` or `windows`.

    - **`os.version`** *string*

        This OPTIONAL property specifies the operating system version, for example `10.0.10586`.

    - **`os.features`** *array*

        This OPTIONAL property specifies an array of strings, each specifying a mandatory OS feature (for example on Windows `win32k`).

    - **`variant`** *string*

        This OPTIONAL property specifies the variant of the CPU, for example `armv6l` to specify a particular CPU variant of the ARM CPU.

    - **`features`** *array*

        This OPTIONAL property specifies an array of strings, each specifying a mandatory CPU feature (for example `sse4` or `aes`).

- **`annotations`** *string-string map*

    This OPTIONAL property contains arbitrary metadata for the manifest list.
    Annotations MUST be a key-value map where both the key and value MUST be strings.
    While the value MUST be present, it MAY be an empty string.
    Keys MUST be unique within this map, and best practice is to namespace the keys.
    Keys SHOULD be named using a reverse domain notation - e.g. `com.example.myKey`.
    Keys using the `org.opencontainers` namespace are reserved and MUST NOT be used by other specifications.
    If there are no annotations then this property MUST either be absent or be an empty map.
    Implementations that are reading/processing manifest lists MUST NOT generate an error if they encounter an unknown annotation key.

    See [Pre-Defined Annotation Keys](manifest.md#pre-defined-annotation-keys).

### Extensibility
Implementations that are reading/processing manifest lists MUST NOT generate an error if they encounter an unknown property.
Instead they MUST ignore unknown properties.

## Example Manifest List

*Example showing a simple manifest list pointing to image manifests for two platforms:*
```json,title=Manifest%20List&mediatype=application/vnd.oci.image.manifest.list.v1%2Bjson
{
  "schemaVersion": 2,
  "mediaType": "application/vnd.oci.image.manifest.list.v1+json",
  "manifests": [
    {
      "mediaType": "application/vnd.oci.image.manifest.v1+json",
      "size": 7143,
      "digest": "sha256:e692418e4cbaf90ca69d05a66403747baa33ee08806650b51fab815ad7fc331f",
      "platform": {
        "architecture": "ppc64le",
        "os": "linux"
      }
    },
    {
      "mediaType": "application/vnd.oci.image.manifest.v1+json",
      "size": 7682,
      "digest": "sha256:5b0bcabd1ed22e9fb1310cf6c2dec7cdef19f0ad69efa1f392e94a4333501270",
      "platform": {
        "architecture": "amd64",
        "os": "linux",
        "features": [
          "sse4"
        ]
      }
    }
  ],
  "annotations": {
    "com.example.key1": "value1",
    "com.example.key2": "value2"
  }
}
```
