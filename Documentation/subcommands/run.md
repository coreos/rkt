# rkt run

## Image Addressing

Images can be run by either their name, their hash, an explicit transport address, or a Docker registry URL.
rkt will automatically [fetch](fetch.md) them if they're not present in the local store.

```
# Run by name
# rkt run coreos.com/etcd:v2.0.0
```

```
# Run by hash
# rkt run sha512-fa1cb92dc276b0f9bedf87981e61ecde
```

```
# Run by ACI address
# rkt run https://github.com/coreos/etcd/releases/download/v2.0.0/etcd-v2.0.0-linux-amd64.aci
```

```
# Run by Docker registry
# rkt --insecure-skip-verify run docker://quay.io/coreos/etcd:v2.0.0
```

## Overriding Executable to launch

Application images include an `exec` field that specifies the executable to launch.
This executable can be overridden by rkt using the `--exec` flag:

```
# rkt --insecure-skip-verify run docker://busybox --exec /bin/date
```

## Passing Arguments

To pass additional arguments to images use the pattern of `image1 -- [image1 flags] --- image2 -- [image2 flags]`.
For example:

```
# rkt run example.com/worker -- --loglevel verbose --- example.com/syncer -- --interval 30s
```

This can be combined with overridden executables:

```
# rkt run example.com/worker --exec /bin/ov -- --loglevel verbose --- example.com/syncer --exec /bin/syncer2 -- --interval 30s
```

## Influencing Environment Variables

To inherit all environment variables from the parent use the `--inherit-env` flag.

To explicitly set individual environment variables use the `--set-env` flag.

The precedence is as follows with the last item replacing previous environment entries:

- Parent environment
- App image environment
- Explicitly set environment

```
# export EXAMPLE_ENV=hello
# export EXAMPLE_OVERRIDE=under
# rkt run --inherit-env --set-env=FOO=bar --set-env=EXAMPLE_OVERRIDE=over example.com/env-printer
EXAMPLE_ENV=hello
FOO=bar
EXAMPLE_OVERRIDE=over
```

## Disable Signature Verification

If desired, `--insecure-skip-verify` can be used to disable this security check:

```
# rkt --insecure-skip-verify run coreos.com/etcd:v2.0.0
rkt: searching for app image coreos.com/etcd:v2.0.0
rkt: fetching image from https://github.com/coreos/etcd/releases/download/v2.0.0/etcd-v2.0.0-linux-amd64.aci
rkt: warning: signature verification has been disabled
...
```

## Mount Volumes into a Pod

Volumes are defined in each ACI and are referenced by name. Volumes can be exposed from the host into the pod (`host`) or initialized as empty storage to be accessed locally within the pod (`empty` pending [rkt #378][rkt #378]). Each volume can be selectively mounted into each application at differing mount points or not mounted into specific apps at all.

[rkt #378]: https://github.com/coreos/rkt/issues/378


### Mounting Host Volumes

For `host` volumes, the `--volume` flag allows you to specify each mount, its type and the location on the host. The volume is then mounted into each app running to the pod based on information defined in the ACI manifest.

For example, let's say we want to read data from the host directory `/opt/tenant1/work` to power a MapReduce-style worker. We'll call this app `example.com/reduce-worker`.

We also want this data to be available to a backup application that runs alongside the worker (in the same pod). We'll call this app 'example.com/worker-backup`. The backup application only needs read-only access to the data.

Below we show the abbreviated manifests for the respective applications (recall that the manifest is bundled into the application's ACI):

```json
{
    "acKind": "ImageManifest",
    "name": "example.com/reduce-worker",
    ...
    "app": {
        ...
        "mountPoints": [
            {
                "name": "work",
                "path": "/var/lib/work",
                "readOnly": false
            }
        ],
    ...
}
```

```json
{
    "acKind": "ImageManifest",
    "name": "example.com/worker-backup",
    ...
    "app": {
        ...
        "mountPoints": [
            {
                "name": "work",
                "path": "/backup",
                "readOnly": true
            }
        ],
    ...
}
```

In this case, both apps reference a volume they call "work", and expect it to be made available at `/var/lib/work` and `/backup` within their respective root filesystems. 

Since they reference the volume using an abstract name rather than a specific source path, the same image can be used on a variety of different hosts without being coupled to the host's filesystem layout.

To tie it all together, we use the `rkt run` command-line to provide them with a volume by this name. Here's what it looks like:

```
# rkt run --volume=work,kind=host,source=/opt/tenant1/work \
  example.com/reduce-worker \
  example.com/worker-backup
```

Now when the pod is running, the two apps will see the host's `/opt/tenant1/work` directory made available at their expected locations.

## Disabling metadata service registration

By default, `rkt run` will register the pod with the [metadata service](https://github.com/coreos/rkt/blob/master/Documentation/subcommands/metadata-service.md).
If the metadata service is not running, it is possible to disable this behavior with `--register-mds=false` command line option.

## Pod Networking

The `run` subcommand features the `--net` argument which takes options to configure the pod's network.

### Default contained networking

When the argument is not given, `--net=default` is automatically assumed and the default contained network network will be loaded.

### Host networking

Simplified, with `--net=host` the apps within the pod will share the network stack and the interfaces with the host machine.

```
# rkt run --net=host coreos.com/etcd:v2.0.0
```

Strictly seen, this is only true when `rkt run` is invoked on the host directly, because the network stack will be inherited from the process that is invoking the `rkt run` command.


### Other Networking Examples

More details about rkt's networking options and examples can be found in the [networking documentation](https://github.com/coreos/rkt/blob/master/Documentation/networking.md)
