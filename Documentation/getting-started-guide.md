# Getting Started with Rocket

The following guide will show you how to build and run a self-contained Go app
using rocket, the reference implementation of the [App Container
Specification](https://github.com/coreos/rocket/tree/master/app-container).

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
    "acVersion": "1.0.0",
    "acKind": "AppManifest",
    "name": "coreos.com/hello-1.0.0",
		"version": "1.0.0",
    "os": "linux",
    "arch": "amd64",
    "exec": [
        "/bin/hello"
    ],
    "ports": [
        {
            "name": "www",
            "protocol": "tcp",
            "port": 5000
        }
    ],
    "annotations": {
        "authors": "Kelsey Hightower <kelsey.hightower@gmail.com>"
    }
}
```

### Validate the application manifest

```
$ actool validate manifest.json
manifest.json: valid AppManifest
```

## Create the rootfs

```
$ mkdir rootfs
$ mkdir rootfs/bin
```

Copy the hello binary

```
$ cp hello rootfs/bin/
```

## Build the application image

```
$ actool build --app-manifest manifest.json rootfs hello.aci
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
