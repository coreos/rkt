# coreos.com/rkt/builder

This container contains all build-time dependencies in order to build rkt.
It currently can be built in: _Debian Sid_.

All commands assume you are running them in your local git checkout of rkt.

## Building coreos.com/rkt/builder using acbuild

Requirements:
- rkt

The file `scripts/acbuild-rkt-builder.sh` contains a simple bash script which generates a Debian Sid tree and creates an ACI ready to build `rkt`.

Running the build script:

```
./scripts/acbuild-rkt-builder.sh
```

Once that is finished there should be a `rkt-builder.aci` file in the current directory.

## Building rkt in rkt

Now that `rkt-builder.aci` has been built you have a container which will compile `rkt`.

Put it into the rkt CAS:

```
rkt fetch --insecure-options=image ./rkt-builder.aci
```

Configure the path to your git checkout of `rkt` and the build output directory respectively:

```
export SRC_DIR=
export BUILDDIR=
mkdir -p $BUILDDIR
```

Start the container which will compile rkt:
```
./scripts/build-rir.sh
```

You should see rkt building in your rkt container, and once it's finished, the output should be in `$BUILD_DIR` on your host.

# Building rkt in rkt one liners (sort of)

If you don't want to bother with acbuild and want a simple one liner that uses rkt to build rkt,  you can install all the dependencies and build rkt from source in one line using bash in a container.
Note that this fetches images from the Docker hub and doesn't check signatures.

Set `SRC_DIR` to the absolute path to your git checkout of `rkt`:

```
export SRC_DIR=
```

Now pick a base OS you want to use, and run the appropriate command.
The build output will be in `${SRC_DIR}/build-rkt-${RKT_VERSION}+git`.

## Debian Sid
```
rkt run \
    --volume rslvconf,kind=host,source=/etc/resolv.conf \
    --mount volume=rslvconf,target=/etc/resolv.conf \
    --volume src-dir,kind=host,source=$SRC_DIR \
    --mount volume=src-dir,target=/opt/rkt \
    --interactive \
    --insecure-options=image \
    docker://debian:sid \
    --exec /bin/bash \
    -- -c 'cd /opt/rkt && ./scripts/install-deps-debian-sid.sh && ./autogen.sh && ./configure --disable-tpm && make'
```

## Fedora 22
```
rkt run \
    --volume rslvconf,kind=host,source=/etc/resolv.conf \
    --mount volume=rslvconf,target=/etc/resolv.conf \
    --volume src-dir,kind=host,source=$SRC_DIR \
    --mount volume=src-dir,target=/opt/rkt \
    --interactive \
    --insecure-options=image \
    docker://fedora:22 \
    --exec /bin/bash \
    -- -c 'cd /opt/rkt && ./scripts/install-deps-fedora-22.sh && ./autogen.sh && ./configure --disable-tpm && make'
```
