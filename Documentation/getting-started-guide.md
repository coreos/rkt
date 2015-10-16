# Getting Started with rkt

The following guide will show you how to build and run a self-contained Go app using
rkt, the reference implementation of the [App Container Specification](https://github.com/appc/spec).

## Create a hello go application

```go
package main

import (
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("request from %v\n", r.RemoteAddr)
		w.Write([]byte("hello\n"))
	})
	log.Fatal(http.ListenAndServe(":5000", nil))
}
```

### Build a statically linked Go binary

Next we need to build our application. We are going to statically link our app
so we can ship an App Container Image with no external dependencies.

With Go 1.3:

```
$ CGO_ENABLED=0 GOOS=linux go build -o hello -a -tags netgo -ldflags '-w' .
```

or, on [Go 1.4](https://github.com/golang/go/issues/9344#issuecomment-69944514):

```
$ CGO_ENABLED=0 GOOS=linux go build -o hello -a -installsuffix cgo .
```

Before proceeding, verify that the produced binary is statically linked:

```
$ file hello
hello: ELF 64-bit LSB executable, x86-64, version 1 (SYSV), statically linked, not stripped
$ ldd hello
	not a dynamic executable
```

## Create the image manifest

Edit: manifest.json

```json
{
    "acKind": "ImageManifest",
    "acVersion": "0.7.0",
    "name": "example.com/hello",
    "labels": [
        {
            "name": "version",
            "value": "0.0.1"
        },
        {
            "name": "arch",
            "value": "amd64"
        },
        {
            "name": "os",
            "value": "linux"
        }
    ],
    "app": {
        "user": "root",
        "group": "root",
        "exec": [
            "/bin/hello"
        ],
        "ports": [
            {
                "name": "www",
                "protocol": "tcp",
                "port": 5000
            }
        ]
    },
    "annotations": [
        {
            "name": "authors",
            "value": "Kelsey Hightower <kelsey.hightower@gmail.com>"
        }
    ]
}
```

### Validate the image manifest

To validate the manifest, we can use `actool`, which is currently provided in [releases in the App Container repository](https://github.com/appc/spec/releases).

```
$ actool --debug validate manifest.json
manifest.json: valid ImageManifest
```

## Create the layout and the rootfs

```
$ mkdir hello-layout/
$ mkdir hello-layout/rootfs
$ mkdir hello-layout/rootfs/bin
```

Copy the image manifest and `hello` binary into the layout:

```
$ cp manifest.json hello-layout/manifest
$ cp hello hello-layout/rootfs/bin/
```

## Build the application image

```
$ actool build hello-layout/ hello-0.0.1-linux-amd64.aci
```

### Validate the application image

```
$ actool --debug validate hello-0.0.1-linux-amd64.aci
hello-0.0.1-linux-amd64.aci: valid app container image
```

## Run

### Launch the metadata service

Start the metadata service from your init system or simply from another terminal:

```
# rkt metadata-service
```

rkt will register pods with the [metadata service](https://github.com/coreos/rkt/blob/master/Documentation/subcommands/metadata-service.md) so they can introspect their environment.

### Launch a local application image

```
# rkt --insecure-skip-verify run hello-0.0.1-linux-amd64.aci
```

Note that `--insecure-skip-verify` is required because, by default, rkt expects our signature to be signed. See the [Signing and Verification Guide](https://github.com/coreos/rkt/blob/master/Documentation/signing-and-verification-guide.md) for more details.

At this point our hello app is running on port 5000 and ready to handle HTTP
requests.

### Test with curl

Open a new terminal and run the following command:

```
$ curl 127.0.0.1:5000
hello
```
