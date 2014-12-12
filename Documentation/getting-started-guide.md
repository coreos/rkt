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

```
$ CGO_ENABLED=0 GOOS=linux go build -a -tags netgo -ldflags '-w' .
```

## Create the application manifest

Edit: manifest.json

```
{
    "acKind": "ImageManifest",
    "acVersion": "0.1.1",
    "name": "coreos.com/hello",
    "labels": [
        {
            "name": "version",
            "val": "1.0.0"
        },
        {
            "name": "arch",
            "val": "amd64"
        },
        {
            "name": "os",
            "val": "linux"
        }
    ],
    "app": {
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
    "annotations": {
        "authors": "Kelsey Hightower <kelsey.hightower@gmail.com>"
    }
}
```

### Validate the image manifest

```
$ actool validate manifest.json
manifest.json: valid ImageManifest
```

## Create the layout and the rootfs

```
$ mkdir hello-layout/
$ mkdir hello-layout/rootfs
$ mkdir hello-layout/rootfs/bin
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
$ actool validate hello.aci
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
