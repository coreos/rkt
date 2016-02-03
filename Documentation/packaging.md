# packaging rkt

This document aims to provide information to package rkt in Linux distributions.
It covers dependencies, file ownership and permissions, and tips to observe packaging policies.

## Build-time dependencies

#### Basic build-time dependencies

- autoconf
- GNU Make
- glibc-static
- bash
- go
- gofmt
- file
- git
- gpg

**Fedora 23**

```
dnf install squashfs-tools glibc-static trousers-devel  @development-tools
```
Note: This assumes a working Go 1.5+ environment. And to test the dev version of `rkt` SELinux must be disabled.

**Ubuntu 15.10**

```
apt-get install build-essential git libc6-dev autoconf squashfs-tools libtspi-dev
```
Note: This assumes a working Go 1.5+ environment.

#### Additional build-time dependencies for the coreos flavor

wget, gpg, mktemp, md5sum, cpio, gzip, unsquashfs, sort

#### Additional build-time dependencies for the kvm flavor

patch, bc

#### Additional build-time dependencies for the src flavor

intltoolize, libtoolize and all systemd dependencies.
The dependencies may differ depending on the version of built systemd.

## Run-time dependencies

#### Basic run-time dependencies

Linux configured with [suitable options](hacking.md#run-time-requirements)

#### Additional run-time dependencies for the host flavor

- systemd >= v222 with systemctl, systemd-shutdown, systemd, systemd-journald.
- bash

## Ownership and permissions of rkt directories

In general, subdirectories of `/var/lib/rkt` should be created with the same ownership and permissions as done by `rkt install`, see [directory list](https://github.com/coreos/rkt/blob/master/rkt/install.go#L44).

The package should create a group `rkt` and `/var/lib/rkt` should belong to group `rkt` with the `setgid` bit set (`chmod g+s`)

When the ownership and permissions of `/var/lib/rkt` are set up correctly, users member of `rkt` should be able to fetch ACIs without using sudo.
However, `root` is still required to run pods.

## Offline builds

By default, rkt compilation will download a CoreOS PXE image from internet and extract some binaries such as `systemd-nspawn` and `bash`.
However, some packaging environments don't allow internet access during the build.

To solve this, you can download the CoreOS PXE image before starting the build process and use the `--with-coreos-local-pxe-image-path` and `--with-coreos-local-pxe-image-systemd-version` parameters.
For more details, see [configure script parameters documentation](build-configure.md).

## Bundling with systemd

Most Linux distributions don't allow the use of prebuilt binaries, or reuse of code that is already otherwise packaged.
systemd falls in this category, as Debian and Fedora already package systemd and rkt needs systemd.

- [Debian Policy Manual, section 4.13 Convenience copies of code](https://www.debian.org/doc/debian-policy/ch-source.html#s-embeddedfiles)
- [Fedora Packaging Guidelines](https://fedoraproject.org/wiki/Packaging:Guidelines#No_inclusion_of_pre-built_binaries_or_libraries)
- [Fedora Packaging Committee](https://www.mail-archive.com/devel@lists.fedoraproject.org/msg88276.html)

In order to avoid build-dependency on systemd in rkt, a build option was added to build stage1 without including binaries that are build from external sources.

```
./configure --with-stage1-flavors=host
```

For more details about configure parameters, see [configure script parameters documentation](build-configure.md).
The generated archive `stage1.aci` will not contain bash, systemd, or any other binaries from external sources.
The binaries embedded in the stage1 archive are all built from the sources in the rkt git repository.
The external binaries needed by `stage1.aci` are copied from the host at run time.
Packages using the `--with-stage1-flavors=host` option must therefore add a run-time dependency on systemd and bash.
Whenever systemd and bash are upgraded on the host, rkt will use the new version at run time.
It becomes the packager's responsibility to test the rkt package whenever a new version of systemd is packaged.

# Other code bundle

rkt uses [Godep](https://github.com/tools/godep) to maintain [a copy of dependencies in its git repository](https://github.com/coreos/rkt/tree/master/Godeps).
