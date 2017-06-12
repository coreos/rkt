# Hacking Guide

## Overview

This guide contains instructions for building artifacts contained in this repository.

### Go

This spec includes several Go packages, and a command line tool considered to be a reference implementation of the OCI image specification.

Prerequisites:
* Go >=1.5
* make

The following make targets are relevant for any work involving the Go packages.

### Linting

The included Go source code is being examined for any linting violations not included in the standard Go compiler. Linting is done using [gometalinter](https://github.com/alecthomas/gometalinter).

Invocation:
```
$ make lint
```

### Tests

This target executes all Go based tests.

Invocation:
```
$ make test
$ make validate-examples
```

### OCI image tool

This target builds the `oci-image-tool` binary.

Invocation:
```
$ make oci-image-tool
```

### Virtual schema http/FileSystem

The `oci-image-tool` uses a virtual [http/FileSystem](https://golang.org/pkg/net/http/#FileSystem) to load the JSON schema files for validating OCI images and/or manifests. The virtual file system is generated using the `esc` tool and compiled into the `oci-image-tool` binary so the JSON schema files don't have to be distributed along with the binary.

Whenever changes are being done in any of the `schema/*.json` files, one must refresh the generated virtual file system. Otherwise schema changes will not be visible inside the `oci-image-tool`.

Prerequisites:
* [esc](https://github.com/mjibson/esc)

Invocation:
```
$ make schema-fs
```

### JSON schema formatting

This target auto-formats all JSON files in the `schema` directory using the `jq` tool.

Prerequisites:
* [jq](https://stedolan.github.io/jq/) >=1.5

Invocation:
```
$ make fmt
```

### OCI image specification PDF/HTML documentation files

This target generates a PDF/HTML version of the OCI image specification.

Prerequisites:
* [Docker](https://www.docker.com/)

Invocation:
```
$ make docs
```

### License header check

This target checks if the source code includes necessary headers.

Invocation:
```
$ make check-license
```

### Update vendored dependencies

This target updates all vendored dependencies to their newest available versions. The `glide` tools is being used for the actual management and `glide-vc` tool is being used for stripping down the vendor directory size.

Prerequisites:
* [glide](https://github.com/Masterminds/glide)
* [glide-vc](https://github.com/sgotti/glide-vc)

Invocation:
```
$ make update-deps
```

### Clean build artifacts

This target cleans all generated/compiled artifacts.

Invocation:
```
$ make clean
```

### Create PNG images from dot files

This target generates PNG image files from DOT source files in the `img` directory.

Prerequisites:
* [graphviz](http://www.graphviz.org/)

Invocation:
```
$ make img/media-types.png
```
