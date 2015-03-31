# Getting Started with Rocket

The following guide will show you how to build and run a self-contained Go app using
rocket, the reference implementation of the [App Container Specification](https://github.com/appc/spec).

## Create a hello go application

```
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

```
{
    "acKind": "ImageManifest",
    "acVersion": "0.2.0",
    "name": "coreos.com/hello",
    "labels": [
        {
            "name": "version",
            "value": "1.0.0"
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
$ actool -debug validate manifest.json
manifest.json: valid ImageManifest
```

## Create the layout and the rootfs

```
$ mkdir hello-layout/
$ mkdir hello-layout/rootfs
$ mkdir hello-layout/rootfs/bin
```

Copy the image manifest

```
$ cp manifest.json hello-layout/manifest
```

Copy the hello binary

```
$ cp hello hello-layout/rootfs/bin/
```

## Build the application image

```
$ actool build hello-layout/ hello.aci
```

### Validate the application image

```
$ actool -debug validate hello.aci
hello.aci: valid app container image
```

## Run

### Launch a local application image

```
$ rkt run hello.aci
```

At this point our hello app is running on port 5000 and ready to handle HTTP
requests.

### Testing with curl

Open a new terminal and run the following command:

```
$ curl 127.0.0.1:5000
hello
```

