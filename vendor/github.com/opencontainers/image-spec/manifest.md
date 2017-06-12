# OCI Image Manifest Specification

There are three main goals of the Image Manifest Specification.
The first goal is content-addressable images, by supporting an image model where the image's configuration can be hashed to generate a unique ID for the image and its components.
The second goal is to allow multi-architecture images, through a "fat manifest" which references image manifests for platform-specific versions of an image.
In OCI, this is codified in a [Manifest List](manifest-list.md).
The third goal is to be translatable to the [OCI Runtime Specification](https://github.com/opencontainers/runtime-spec).

This section defines the `application/vnd.oci.image.manifest.v1+json` [media type](media-types.md).

# Image Manifest

Unlike the [Manifest List](manifest-list.md), which contains information about a set of images that can span a variety of architectures and operating systems, an image manifest provides a configuration and set of layers for a single container image for a specific architecture and operating system.

## *Image Manifest* Property Descriptions

- **`schemaVersion`** *int*

  This REQUIRED property specifies the image manifest schema version.
  For this version of the specification, this MUST be `2` to ensure backward compatibility with older versions of Docker. The value of this field will not change. This field MAY be removed in a future version of the specification.

- **`mediaType`** *string*

    This REQUIRED property contains the media type of the image manifest.
    For this version of the specification, this MUST be set to `application/vnd.oci.image.manifest.v1+json`.
    For the media type(s) that this is compatible with see the [matrix](media-types.md#compatibility-matrix).

- **`config`** *[descriptor](descriptor.md)*

    This REQUIRED property references a configuration object for a container, by digest.
    Beyond the [descriptor requirements](descriptor.md#properties), the value has the following additional restrictions:

    - **`mediaType`** *string*

        This [descriptor property](descriptor.md#properties) has additional restrictions for `config`.
        Implementations MUST support at least the following media types:

        - [`application/vnd.oci.image.config.v1+json`](config.md)

        Manifests concerned with portability SHOULD use one of the above media types.

- **`layers`** *array*

    Each item in the array MUST be a [descriptor](descriptor.md).
    The array MUST have the base image at index 0.
    Subsequent layers MUST then follow in stack order (i.e. from `layers[0]` to `layers[len(layers)-1]`).
    The final filesystem layout MUST match the result of [applying](layer.md#applying) the layers to an empty directory.

    Beyond the [descriptor requirements](descriptor.md#properties), the value has the following additional restrictions:

    - **`mediaType`** *string*

        This [descriptor property](descriptor.md#properties) has additional restrictions for `layers[]`.
        Implementations MUST support at least the following media types:

        - [`application/vnd.oci.image.layer.v1.tar+gzip`](layer.md)
        - [`application/vnd.oci.image.layer.nondistributable.v1.tar+gzip`](layer.md#non-distributable-layers)

        Manifests concerned with portability SHOULD use one of the above media types.

- **`annotations`** *string-string map*

    This OPTIONAL property contains arbitrary metadata for the image manifest.
    Annotations MUST be a key-value map where both the key and value MUST be strings.
    While the value MUST be present, it MAY be an empty string.
    Keys MUST be unique within this map, and best practice is to namespace the keys.
    Keys SHOULD be named using a reverse domain notation - e.g. `com.example.myKey`.
    Keys using the `org.opencontainers` namespace are reserved and MUST NOT be used by other specifications.
    If there are no annotations then this property MUST either be absent or be an empty map.
    Implementations that are reading/processing the image manifest MUST NOT generate an error if they encounter an unknown annotation key.

    See [Pre-Defined Annotation Keys](#pre-defined-annotation-keys).

### Extensibility
Implementations that are reading/processing image manifests MUST NOT generate an error if they encounter an unknown property.
Instead they MUST ignore unknown properties.

## Example Image Manifest

*Example showing an image manifest:*
```json,title=Manifest&mediatype=application/vnd.oci.image.manifest.v1%2Bjson
{
  "schemaVersion": 2,
  "mediaType": "application/vnd.oci.image.manifest.v1+json",
  "config": {
    "mediaType": "application/vnd.oci.image.config.v1+json",
    "size": 7023,
    "digest": "sha256:b5b2b2c507a0944348e0303114d8d93aaaa081732b86451d9bce1f432a537bc7"
  },
  "layers": [
    {
      "mediaType": "application/vnd.oci.image.layer.v1.tar+gzip",
      "size": 32654,
      "digest": "sha256:e692418e4cbaf90ca69d05a66403747baa33ee08806650b51fab815ad7fc331f"
    },
    {
      "mediaType": "application/vnd.oci.image.layer.v1.tar+gzip",
      "size": 16724,
      "digest": "sha256:3c3a4604a545cdc127456d94e421cd355bca5b528f4a9c1905b15da2eb4a4c6b"
    },
    {
      "mediaType": "application/vnd.oci.image.layer.v1.tar+gzip",
      "size": 73109,
      "digest": "sha256:ec4b8955958665577945c89419d1af06b5f7636b4ac3da7f12184802ad867736"
    }
  ],
  "annotations": {
    "com.example.key1": "value1",
    "com.example.key2": "value2"
  }
}
```

# Pre-Defined Annotation Keys
This specification defines the following annotation keys, which MAY be used by manifest list and image manifest authors:
* **org.opencontainers.created** date on which the image was built (string, date-time as defined by [RFC 3339](https://tools.ietf.org/html/rfc3339#section-5.6)).
* **org.opencontainers.authors** contact details of the people or organization responsible for the image (freeform string)
* **org.opencontainers.homepage** URL to find more information on the image (string, must be a URL with scheme HTTP or HTTPS)
* **org.opencontainers.documentation** URL to get documentation on the image (string, must be a URL with scheme HTTP or HTTPS)
