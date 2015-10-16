# rkt prepare

rkt can prepare images to run in a pod. This means it will fetch (if necessary) the images, extract them in its internal tree store, and allocate a pod UUID. If overlay fs is not supported or disabled, it will also copy the tree in the pod rootfs.

In this way, the pod is ready to be launched immediately by the [run-prepared](run-prepared.md) command.

Running `rkt prepare` + `rkt run-prepared` is semantically equivalent to running [rkt run](run.md).
Therefore, the supported arguments are mostly the same as in `run` except runtime arguments like `--interactive` or `--mds-register`.

## Example

```
# rkt prepare coreos.com/etcd:v2.0.10
rkt prepare coreos.com/etcd:v2.0.10
rkt: using image from local store for image name coreos.com/rkt/stage1-coreos:0.9.0
rkt: searching for app image coreos.com/etcd:v2.0.10
rkt: remote fetching from url https://github.com/coreos/etcd/releases/download/v2.0.10/etcd-v2.0.10-linux-amd64.aci
prefix: "coreos.com/etcd"
key: "https://coreos.com/dist/pubkeys/aci-pubkeys.gpg"
gpg key fingerprint is: 8B86 DE38 890D DB72 9186  7B02 5210 BD88 8818 2190
	CoreOS ACI Builder <release@coreos.com>
Key "https://coreos.com/dist/pubkeys/aci-pubkeys.gpg" already in the keystore
Downloading signature from https://github.com/coreos/etcd/releases/download/v2.0.10/etcd-v2.0.10-linux-amd64.aci.asc
Downloading signature: [=======================================] 819 B/819 B
Downloading ACI: [=============================================] 3.79 MB/3.79 MB
rkt: signature verified:
  CoreOS ACI Builder <release@coreos.com>
c9fad0e6-8236-4fc2-ad17-55d0a4c7d742
```
