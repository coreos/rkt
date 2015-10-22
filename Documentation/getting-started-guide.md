# Getting Started with rkt

The following guide will show you how to build and run a self-contained Go app using
rkt, the reference implementation of the [App Container Specification](https://github.com/appc/spec).
If you're not on Linux, you should do all of this inside [the rkt Vagrant](https://github.com/coreos/rkt#trying-out-rkt-using-vagrant).

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

With Go 1.3 or 1.5:

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

## Create the image

To create the image, we can use `acbuild`, which can be downloaded via one of
the [releases in the acbuild
repository](https://github.com/appc/acbuild/releases)

The following commands (run as root) will create an ACI containing our
application and important metadata.

```bash
acbuild begin
acbuild set-name example.com/hello
acbuild copy hello /bin/hello
acbuild set-exec /bin/hello
acbuild port add www tcp 5000

acbuild label add version 0.0.1
acbuild label add arch amd64
acbuild label add os linux
acbuild annotation add authors "Kelsey Hightower <kelsey.hightower@gmail.com>"

acbuild write hello-0.0.1-linux-amd64.aci
acbuild end
```

## Run

### Launch the metadata service

Start the metadata service from your init system or simply from another terminal:

```
# rkt metadata-service
```
Notice that the `#` indicates that this should be run as root.

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

#### When curl Fails to Connect

If you're running in Vagrant, the above may not work. You might see this instead:

```
$ curl 127.0.0.1:5000
curl: (7) Failed to connect to 127.0.0.1 port 5000: Connection refused
```

Instead, use `rkt list` to find out what IP to use:

```
# rkt list
UUID		APP	IMAGE NAME		STATE	NETWORKS
885876b0	hello	example.com/hello:0.0.1	running	default:ip4=172.16.28.2
```

Then you can `curl` that IP:
```
$ curl 172.16.28.2:5000
hello
```
