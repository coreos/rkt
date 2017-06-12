# OCI Image Configuration

An OCI *Image* is an ordered collection of root filesystem changes and the corresponding execution parameters for use within a container runtime.
This specification outlines the JSON format describing images for use with a container runtime and execution tool and its relationship to filesystem changesets, described in [Layers](layer.md).

This section defines the `application/vnd.oci.image.config.v1+json` [media type](media-types.md).

## Terminology

This specification uses the following terms:

### [Layer](layer.md)

Image filesystems are composed of *layers*.
Each layer represents a set of filesystem changes in a tar-based [layer format](layer.md), recording files to be added, changed, or deleted relative to its parent layer.
Layers do not have configuration metadata such as environment variables or default arguments - these are properties of the image as a whole rather than any particular layer.
Using a layer-based or union filesystem such as AUFS, or by computing the diff from filesystem snapshots, the filesystem changeset can be used to present a series of image layers as if they were one cohesive filesystem.

### Image JSON

Each image has an associated JSON structure which describes some basic information about the image such as date created, author, and the ID of its parent image as well as execution/runtime configuration like its entrypoint, default arguments, CPU/memory shares, networking, and volumes.
The JSON structure also references a cryptographic hash of each layer used by the image, and provides history information for those layers.
This JSON is considered to be immutable, because changing it would change the computed [ImageID](#imageid).
Changing it means creating a new derived image, instead of changing the existing image.

### Layer DiffID

A layer DiffID is a SHA256 digest over the layer's uncompressed tar archive and serialized in the descriptor digest format, e.g., `sha256:a9561eb1b190625c9adb5a9513e72c4dedafc1cb2d4c5236c9a6957ec7dfd5a9`.
Layers must be packed and unpacked reproducibly to avoid changing the layer DiffID, for example by using tar-split to save the tar headers.

NOTE: the DiffID is different than the digest in the manifest list because the manifest digest is taken over the gzipped layer for `application/vnd.oci.image.layer.v1.tar+gzip` types.

### Layer ChainID

For convenience, it is sometimes useful to refer to a stack of layers with a single identifier.
This is called a `ChainID`.
For a single layer (or the layer at the bottom of a stack), the
`ChainID` is equal to the layer's `DiffID`.
Otherwise the `ChainID` is given by the formula:
`ChainID(layerN) = SHA256hex(ChainID(layerN-1) + " " + DiffID(layerN))`.

### ImageID

Each image's ID is given by the SHA256 hash of its [configuration JSON](#image-json).
It is represented as a hexadecimal encoding of 256 bits, e.g., `sha256:a9561eb1b190625c9adb5a9513e72c4dedafc1cb2d4c5236c9a6957ec7dfd5a9`.
Since the [configuration JSON](#image-json) that gets hashed references hashes of each layer in the image, this formulation of the ImageID makes images content-addresable.

## Properties

Note: Any OPTIONAL field MAY also be set to null, which is equivalent to being absent.

- **created** *string*, OPTIONAL

  An ISO-8601 formatted combined date and time at which the image was created.

- **author** *string*, OPTIONAL

  Gives the name and/or email address of the person or entity which created and is responsible for maintaining the image.

- **architecture** *string*, REQUIRED

  The CPU architecture which the binaries in this image are built to run on.
  Possible values include: `386`, `amd64`, `arm`.
  More values may be supported in the future and any of these may or may not be supported by a given container runtime implementation.
  New entries SHOULD be submitted to this specification for standardization and be inspired by the [Go language documentation for $GOOS and $GOARCH](https://golang.org/doc/install/source#environment).

- **os** *string*, REQUIRED

  The name of the operating system which the image is built to run on.
  Possible values include: `darwin`, `freebsd`, `linux`.
  More values may be supported in the future and any of these may or may not be supported by a given container runtime implementation.
  New entries SHOULD be submitted to this specification for standardization and be inspired by the [Go language documentation for $GOOS and $GOARCH](https://golang.org/doc/install/source#environment).

- **config** *object*, OPTIONAL

  The execution parameters which should be used as a base when running a container using the image.
  This field can be `null`, in which case any execution parameters should be specified at creation of the container.

   - **User** *string*, OPTIONAL

     The username or UID which the process in the container should run as.
     This acts as a default value to use when the value is not specified when creating a container.
     All of the following are valid: `user`, `uid`, `user:group`, `uid:gid`, `uid:group`, `user:gid`
     If `group`/`gid` is not specified, the default group and supplementary groups of the given `user`/`uid` in `/etc/passwd` from the container are applied.

   - **Memory** *integer*, OPTIONAL

     Memory limit (in bytes).
     This acts as a default value to use when the value is not specified when creating a container.

   - **MemorySwap** *integer*, OPTIONAL

     Total memory usage (memory + swap); set to `-1` to disable swap.
     This acts as a default value to use when the value is not specified when creating a container.

   - **CpuShares** *integer*, OPTIONAL

     CPU shares (relative weight vs. other containers).
     This acts as a default value to use when the value is not specified when creating a container.

   - **ExposedPorts** *object*, OPTIONAL

     A set of ports to expose from a container running this image.
     Its keys can be in the format of:
`port/tcp`, `port/udp`, `port` with the default protocol being `tcp` if not specified.
     These values act as defaults and are merged with any specified when creating a container.
     **NOTE:** This JSON structure value is unusual because it is a direct JSON serialization of the Go type `map[string]struct{}` and is represented in JSON as an object mapping its keys to an empty object.

   - **Env** *array of strings*, OPTIONAL

     Entries are in the format of `VARNAME="var value"`.
     These values act as defaults and are merged with any specified when creating a container.

   - **Entrypoint** *array of strings*, OPTIONAL

     A list of arguments to use as the command to execute when the container starts.
     This value acts as a default and is replaced by an entrypoint specified when creating a container.

   - **Cmd** *array of strings*, OPTIONAL

     Default arguments to the entrypoint of the container.
     These values act as defaults and are replaced with any specified when creating a container.
     If an `Entrypoint` value is not specified, then the first entry of the `Cmd` array should be interpreted as the executable to run.

   - **Volumes** *object*, OPTIONAL

     A set of directories which should be created as data volumes in a container running this image.
     If a file or folder exists within the image with the same path as a data volume, that file or folder is replaced with the data volume and is never merged.
     **NOTE:** This JSON structure value is unusual because it is a direct JSON serialization of the Go type `map[string]struct{}` and is represented in JSON as an object mapping its keys to an empty object.

   - **WorkingDir** *string*, OPTIONAL

     Sets the current working directory of the entrypoint process in the container.
     This value acts as a default and is replaced by a working directory specified when creating a container.

- **rootfs** *object*, REQUIRED

   The rootfs key references the layer content addresses used by the image.
   This makes the image config hash depend on the filesystem hash.

    - **type** *string*, REQUIRED

       MUST be set to `layers`.
       Implementations MUST generate an error if they encounter a unknown value while verifying or unpacking an image.

    - **diff_ids** *array*, REQUIRED

       An array of layer content hashes (`DiffIDs`), in order from first to last.

- **history** *array of objects*, OPTIONAL

  Describes the history of each layer.
  The array is ordered from first to last.
  The object has the following fields:

    - **created** *string*, OPTIONAL

       Creation time, expressed as an ISO-8601 formatted combined date and time

    - **author** *string*, OPTIONAL

       The author of the build point.

    - **created_by** *string*, OPTIONAL

       The command which created the layer.

    - **comment** *string*, OPTIONAL

       A custom message set when creating the layer.

    - **empty_layer** *boolean*, OPTIONAL

       This field is used to mark if the history item created a filesystem diff.
       It is set to true if this history item doesn't correspond to an actual layer in the rootfs section (for example, a command like ENV which results in no change to the filesystem).

Any extra fields in the Image JSON struct are considered implementation specific and should be ignored by any implementations which are unable to interpret them.

Whitespace is OPTIONAL and implementations MAY have compact JSON with no whitespace.

## Example

Here is an example image configuration JSON document:

```json,title=Image%20JSON&mediatype=application/vnd.oci.image.config.v1%2Bjson
{
    "created": "2015-10-31T22:22:56.015925234Z",
    "author": "Alyssa P. Hacker <alyspdev@example.com>",
    "architecture": "amd64",
    "os": "linux",
    "config": {
        "User": "alice",
        "Memory": 2048,
        "MemorySwap": 4096,
        "CpuShares": 8,
        "ExposedPorts": {
            "8080/tcp": {}
        },
        "Env": [
            "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
            "FOO=oci_is_a",
            "BAR=well_written_spec"
        ],
        "Entrypoint": [
            "/bin/my-app-binary"
        ],
        "Cmd": [
            "--foreground",
            "--config",
            "/etc/my-app.d/default.cfg"
        ],
        "Volumes": {
            "/var/job-result-data": {},
            "/var/log/my-app-logs": {}
        },
        "WorkingDir": "/home/alice"
    },
    "rootfs": {
      "diff_ids": [
        "sha256:c6f988f4874bb0add23a778f753c65efe992244e148a1d2ec2a8b664fb66bbd1",
        "sha256:5f70bf18a086007016e948b04aed3b82103a36bea41755b6cddfaf10ace3c6ef"
      ],
      "type": "layers"
    },
    "history": [
      {
        "created": "2015-10-31T22:22:54.690851953Z",
        "created_by": "/bin/sh -c #(nop) ADD file:a3bc1e842b69636f9df5256c49c5374fb4eef1e281fe3f282c65fb853ee171c5 in /"
      },
      {
        "created": "2015-10-31T22:22:55.613815829Z",
        "created_by": "/bin/sh -c #(nop) CMD [\"sh\"]",
        "empty_layer": true
      }
    ]
}
```
