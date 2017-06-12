# oci-image-tool

A tool for working with OCI images

## Building

This project uses the Go programming language and is tested with the [Go
compiler](https://golang.org/dl/). (Results with gccgo may vary)

Install from a particular version of this repo (i.e. the v0.4.0 tag):

```bash
go get -u -d github.com/opencontainers/image-spec
cd $GOPATH/src/github.com/opencontainers/image-spec
git checkout -f v0.4.0
go install github.com/opencontainers/image-spec/cmd/oci-image-tool
```

While the tool may be `go get`'able, it is encouraged that it is only used per
tagged releases.

## Usage

See the tool's own `--help` output.
