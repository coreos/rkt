# OCI Content Descriptors

An OCI image consists of several different components, arranged in a [Merkle Directed Acyclic Graph (DAG)](https://en.wikipedia.org/wiki/Merkle_tree).
References between components in the graph are expressed through _Content Descriptors_.
A Content Descriptor (or simply _Descriptor_) describes the disposition of the targeted content.
A Content Descriptor includes the type of the content, a content identifier (_digest_), and the byte-size of the raw content.

Descriptors SHOULD be embedded in other formats to securely reference external content.

Other formats SHOULD use descriptors to securely reference external content.

This section defines the `application/vnd.oci.descriptor.v1+json` [media type](media-types.md).

## Properties

A descriptor consists of a set of properties encapsulated in key-value fields.

The following fields contain the primary properties that constitute a Descriptor:

- **`mediaType`** *string*

  This REQUIRED property contains the MIME type of the referenced content.

  The OCI image specification defines [several of its own MIME types](media-types.md) for resources defined in the specification.

- **`digest`** *string*

  This REQUIRED property is the _digest_ of the targeted content, conforming to the requirements outlined in [Digests and Verification](#digests-and-verification).
  Retrieved content SHOULD be verified against this digest when consumed via untrusted sources.

- **`size`** *int64*

  This REQUIRED property specifies the size, in bytes, of the raw content.
  This property exists so that a client will have an expected size for the content before processing.
  If the length of the retrieved content does not match the specified length, the content SHOULD NOT be trusted.

- **`urls`** *array*

  This OPTIONAL property specifies a list of URLs from which this object MAY be downloaded.

### Reserved

The following field keys are reserved and MUST NOT be used by other specifications.

- **`data`** *string*

  This key is RESERVED for future versions of the specification.

All other fields may be included in other OCI specifications.
Extended _Descriptor_ field additions proposed in other OCI specifications SHOULD first be considered for addition into this specification.

## Digests and Verification

The _digest_ property of a Descriptor acts as a content identifier, enabling [content addressability](http://en.wikipedia.org/wiki/Content-addressable_storage).
It uniquely identifies content by taking a [collision-resistant hash](https://en.wikipedia.org/wiki/Cryptographic_hash_function) of the bytes.
If the identifier can be communicated in a secure manner, one can retrieve the content from an insecure source, calculate the digest independently, and be certain that the correct content was obtained.

The value of the digest property, the _digest string_, is a serialized hash result, consisting of an _algorithm_ portion and a _hex_ portion.
The algorithm identifies the methodology used to calculate the digest; the hex portion is the lowercase hex-encoded result of the hash.

The digest string MUST match the following grammar:

```
digest      := algorithm ":" hex
algorithm   := /[a-z0-9_+.-]+/
hex         := /[a-f0-9]+/
```

Some example digest strings include the following:

digest                                                                            | description                                   |
----------------------------------------------------------------------------------|------------------------------------------------
sha256:6c3c624b58dbbcd3c0dd82b4c53f04194d1247c6eebdaab7c610cf7d66709b3b           | Common sha256 based digest                    |

Before consuming content targeted by a descriptor from untrusted sources, the byte content SHOULD be verified against the digest.
Before calculating the digest, the size of the content SHOULD be verified to reduce hash collision space.
Heavy processing before calculating a hash SHOULD be avoided.
Implementations MAY employ some canonicalization of the underlying content to ensure stable content identifiers.

### Algorithms

While the _algorithm_ component of the digest does allow one to utilize a wide variety of algorithms, compliant implementations SHOULD use [SHA-256](#sha-256).

Let's use a simple example in pseudo-code to demonstrate a digest calculation:
A _digest_ is calculated by the following pseudo-code, where `H` is the selected hash algorithm, identified by string `<alg>`:
```
let ID(C) = Descriptor.digest
let C = <bytes>
let D = '<alg>:' + EncodeHex(H(C))
let verified = ID(C) == D
```
Above, we define the content identifier as `ID(C)`, extracted from the `Descriptor.digest` field.
Content `C` is a string of bytes.
Function `H` returns the hash of `C` in bytes and is passed to function `EncodeHex` to obtain the _digest_.
The result `verified` is true if `ID(C)` is equal to `D`, confirming that `C` is the content identified by `D`.
After verification, the following is true:

```
D == ID(C) == '<alg>:' + EncodeHex(H(C))
```

The _digest_ is confirmed as the content identifier by independently calculating the _digest_.

#### SHA-256

[SHA-256](https://tools.ietf.org/html/rfc4634#page-7) is a collision-resistant hash function, chosen for ubiquity, reasonable size and secure characteristics.
Implementations MUST implement SHA-256 digest verification for use in descriptors.

## Examples

The following example describes a [_Manifest_](manifest.md#image-manifest) with a content identifier of "sha256:5b0bcabd1ed22e9fb1310cf6c2dec7cdef19f0ad69efa1f392e94a4333501270", of size 7682 bytes:

```json,title=Content%20Descriptor&mediatype=application/vnd.oci.descriptor.v1%2Bjson
{
  "mediaType": "application/vnd.oci.image.manifest.v1+json",
  "size": 7682,
  "digest": "sha256:5b0bcabd1ed22e9fb1310cf6c2dec7cdef19f0ad69efa1f392e94a4333501270"
}
```
