# goaci

`goaci` is a simple command-line tool to build go projects into ACIs which confirm to the [app container specification][appc-spec].

[appc-spec]: https://github.com/appc/spec

## Usage

Use `goaci` as you would `go get`:

	$ goaci github.com/coreos/etcd
	Wrote etcd.aci
	$ actool -debug validate etcd.aci
	etcd.aci: valid app container image

`goaci` provides options for specifying assets, adding arguments for an application, selecting binary is going to be packaged in final ACI and so on. Use --help to read about them.

## How it works

`goaci` creates a temporary directory and uses it as a `GOPATH` (unless it is overridden with `--go-path` option); it then `go get`s the specified package and compiles it statically.
Then it generates an image manifest (using mostly default values) and leverages the [appc/spec](https://github.com/appc/spec) libraries to construct an ACI.

## TODO

Lots, check out https://github.com/appc/goaci/issues
