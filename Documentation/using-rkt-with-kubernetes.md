# Using rkt with Kubernetes (aka "rktnetes")

[Kubernetes](http://kubernetes.io) is a system for managing containerized applications across a cluster of machines.
Kubernetes runs all applications in containers.
In the default setup, this is performed using the Docker engine, but Kubernetes also features support for using rkt as its container runtime backend.
This allows a Kubernetes cluster to leverage some of rkt's security features and native pod support.

## Configuring rkt as the Kubernetes container runtime

The container runtime is configured at the _kubelet_ level.
The kubelet is the agent that runs on each machine to manage containers.
The kubelet provides several flags to use rkt as the container runtime:

- `--container-runtime=rkt` Sets the node's container runtime to rkt.
- `--rkt-api-endpoint=HOST:PORT` Sets the endpoint of the rkt API service. Default to `localhost:15441`.
- `--rkt-path=PATH_TO_RKT_BINARY` Sets the path of the rkt binary. If empty, it will search for rkt in `$PATH`.
- `--rkt-stage1-image=STAGE1_NAME` Sets the name of the stage1 image, e.g. `coreos.com/rkt/stage1-coreos`. If not set, the default stage1 image (`coreos.com/rkt/stage1-coreos`) is used.

Check the [rktnetes getting started guide](http://kubernetes.io/docs/getting-started-guides/rkt/) for information about setting up and using a rktnetes cluster.

### Current Status

Integration of rkt as a container runtime was officially [announced in the Kubernetes 1.3 release](http://blog.kubernetes.io/2016/07/rktnetes-brings-rkt-container-engine-to-Kubernetes.html).
Known issues and tips for using rkt with Kubernetes can be found in the [rktnetes notes](http://kubernetes.io/docs/getting-started-guides/rkt/notes/).
