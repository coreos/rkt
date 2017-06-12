% OCI(1) OCI-IMAGE-TOOL User Manuals
% OCI Community
% JULY 2016
# NAME
oci-image-tool-create-runtime-bundle \- Create an OCI runtime bundle

# SYNOPSIS
**oci-image-tool create-runtime-bundle** [src] [dest] [flags]

# DESCRIPTION
`oci-image-tool create-runtime-bundle` validates an application/vnd.oci.image.manifest.v1+json and unpacks its layered filesystem to `dest/rootfs`, although the target directory is configurable with `--rootfs`. See **oci-image-tool-unpack**(1) for more details on this process.

Also translates the referenced config from application/vnd.oci.image.config.v1+json to a
runtime-spec-compatible `dest/config.json`.

# OPTIONS
**--help**
  Print usage statement

**--ref**
  The ref pointing to the manifest of the OCI image. This must be present in the "refs" subdirectory of the image. (default "v1.0")

**--rootfs**
  A directory representing the root filesystem of the container in the OCI runtime bundle. It is strongly recommended to keep the default value. (default "rootfs")

**--type**
  Type of the file to unpack. If unset, oci-image-tool will try to auto-detect the type. One of "imageLayout,image"

# EXAMPLES
```
$ skopeo copy docker://busybox oci:busybox-oci
$ mkdir busybox-bundle
$ oci-image-tool create-runtime-bundle --ref latest busybox-oci busybox-bundle
$ cd busybox-bundle && sudo runc run busybox
[...]
```

# SEE ALSO
**oci-image-tool**(1), **runc**(1), **skopeo**(1)

# HISTORY
July 2016, Originally compiled by Antonio Murdaca (runcom at redhat dot com)
