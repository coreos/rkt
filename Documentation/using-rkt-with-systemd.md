# Using rkt with systemd

**work in progress**

This document describes how to use `rkt` with `systemd`.

## Overview

`rkt` is designed and intended to be used with init systems like [`systemd`](http://www.freedesktop.org/wiki/Software/systemd/).
Since `rkt` consists only of a simple CLI that directly executes processes and writes to stdout/stderr (i.e. it does not daemonize), the lifecycle of `rkt` pods can be directly managed by `systemd`.
Consequently, standard `systemd` idioms like `systemctl start` and `systemctl stop` work out of the box.

## systemd-run

To start a daemonized container from the command line, use [`systemd-run`](http://www.freedesktop.org/software/systemd/man/systemd-run.html):

```
# systemd-run rkt run coreos.com/etcd:v2.0.10
Running as unit run-29075.service.
```

This creates a transient systemd unit on which you can use standard systemd tools:

```
$ systemctl status run-3247.service
● run-3247.service - /home/iaguis/work/coreos/go/src/github.com/coreos/rkt/build-rkt/bin/rkt run coreos.com/etcd:v2.0.10
   Loaded: loaded
  Drop-In: /run/systemd/system/run-3247.service.d
           └─50-Description.conf, 50-ExecStart.conf
   Active: active (running) since Mon 2015-10-26 17:38:06 CET; 41s ago
 Main PID: 3254 (ld-linux-x86-64)
   CGroup: /system.slice/run-3247.service
           ├─3254 stage1/rootfs/usr/lib/ld-linux-x86-64.so.2 stage1/rootfs/usr/bin/systemd-nspawn --boot --register=true --link-jou...
           ├─3321 /usr/lib/systemd/systemd --default-standard-output=tty --log-target=null --log-level=warning --show-status=0
           └─system.slice
             ├─etcd.service
             │ └─3326 /etcd
             └─systemd-journald.service
               └─3322 /usr/lib/systemd/systemd-journald

Oct 26 17:38:11 locke rkt[3254]: [25966.375411] etcd[4]: 2015/10/26 16:38:11 raft: ce2a822cea30bfca became follower at term 0
Oct 26 17:38:11 locke rkt[3254]: [25966.375685] etcd[4]: 2015/10/26 16:38:11 raft: newRaft ce2a822cea30bfca [peers: [], ter...term: 0]
Oct 26 17:38:11 locke rkt[3254]: [25966.375942] etcd[4]: 2015/10/26 16:38:11 raft: ce2a822cea30bfca became follower at term 1
Oct 26 17:38:11 locke rkt[3254]: [25966.382092] etcd[4]: 2015/10/26 16:38:11 etcdserver: added local member ce2a822cea30bfc...22e8b2ae
Oct 26 17:38:12 locke rkt[3254]: [25967.675888] etcd[4]: 2015/10/26 16:38:12 raft: ce2a822cea30bfca is starting a new elect...t term 1
Oct 26 17:38:12 locke rkt[3254]: [25967.676920] etcd[4]: 2015/10/26 16:38:12 raft: ce2a822cea30bfca became candidate at term 2
Oct 26 17:38:12 locke rkt[3254]: [25967.677563] etcd[4]: 2015/10/26 16:38:12 raft: ce2a822cea30bfca received vote from ce2a...t term 2
Oct 26 17:38:12 locke rkt[3254]: [25967.678296] etcd[4]: 2015/10/26 16:38:12 raft: ce2a822cea30bfca became leader at term 2
Oct 26 17:38:12 locke rkt[3254]: [25967.678780] etcd[4]: 2015/10/26 16:38:12 raft.node: ce2a822cea30bfca elected leader ce2...t term 2
Oct 26 17:38:12 locke rkt[3254]: [25967.682464] etcd[4]: 2015/10/26 16:38:12 etcdserver: published {Name:default ClientURLs...22e8b2ae
Hint: Some lines were ellipsized, use -l to show in full.
```

Since every pod is registered with machined with a machine name of the form `rkt-$UUID`, you can use the systemd tools to stop pods or get their logs:

```
# journalctl -M rkt-f0261476-7044-4a84-b729-e0f7a47dcffe
...
# machinectl
MACHINE                                  CLASS     SERVICE
rkt-f0261476-7044-4a84-b729-e0f7a47dcffe container nspawn

1 machines listed.
# machinectl poweroff rkt-f0261476-7044-4a84-b729-e0f7a47dcffe
# machinectl
MACHINE CLASS SERVICE

0 machines listed.
```

## Simple Unit File

The following is a simple example of a unit file using `rkt` to run an `etcd` instance:

```
[Unit]
Description=etcd

[Service]
ExecStart=/usr/bin/rkt run --mds-register=false coreos.com/etcd:v2.0.10
KillMode=mixed
Restart=always
```

This unit can now be managed using the standard `systemctl` commands:

```
systemctl start etcd.service
systemctl stop etcd.service
systemctl restart etcd.service
systemctl enable etcd.service
systemctl disable etcd.service
```

`ExecStop` clause is not required - setting [`KillMode=mixed`](http://www.freedesktop.org/software/systemd/man/systemd.kill.html#KillMode=) means that running `systemctl stop etcd.service` will send `SIGTERM` to `stage1`'s `systemd`, which in turn will initiate orderly shutdown inside the pod.
In case something goes wrong with container's shutdown, to ensure no processes are left around, `SIGKILL` will be sent to the remaining service processes after a timeout.

## Advanced Unit File

A more advanced unit example takes advantage of a few convenient `systemd` features:

1. Inheriting environment variables specified in the unit with `--inherit-env`. This functionality keeps your units clear and concise instead of layering on a ton of flags to `rkt run`.
2. Using the dependency graph to start our pod after networking has come online. This is helpful if your application requires outside connectivity to fetch remote configuration (for example, from `etcd`).
3. Set resource limits for this `rkt` pod. This can also be done in the unit instead of `rkt run`.

Here is what it looks like all together:

```
[Unit]
# Metadata
Description=MyApp
Documentation=https://myapp.com/docs/1.3.4
# Wait for networking
Requires=network-online.target
After=network-online.target

[Service]
# Resource limits
CPUShares=512
MemoryLimit=1G
# Env vars
Environment=HTTP_PROXY=192.0.2.3:5000
Environment=STORAGE_PATH=/opt/myapp
Environment=TMPDIR=/var/tmp
# Fetch the app (not strictly required, `rkt run` will fetch the image if there is not one)
ExecStartPre=/usr/bin/rkt fetch myapp.com/myapp-1.3.4
# Start the app
ExecStart=/usr/bin/rkt run --inherit-env --port=http:8888 myapp.com/myapp-1.3.4
KillMode=mixed
Restart=always
```

rkt must be the main process of the service in order to support [isolators](https://github.com/appc/spec/blob/master/spec/ace.md#isolators) correctly and to be well-integrated with [systemd-machined](http://www.freedesktop.org/software/systemd/man/systemd-machined.service.html).
To ensure that rkt is the main process of the service, the pattern `/bin/sh -c "foo ; rkt run ..."` should be avoided because in that case the main process would be sh.

In most cases, the parameters `Environment=` and `ExecStartPre=` can simply be used instead of starting a shell.
If it is not possible, use exec to ensure rkt is not started as a new process:

```
ExecStart=/bin/sh -c "foo ; exec rkt run ..."
```

## Socket-activated service

`rkt` supports [socket-activated services](http://www.freedesktop.org/software/systemd/man/sd_listen_fds.html).
This means systemd will listen on a port on behalf of a container and when it receives a connection to that port, it will start the container and the app inside will handle the connection.
Note that your application needs to be able to accept sockets from systemd's native socket passing interface.

To make socket activation work, you need to add to your application manifest a [socket-activated port](https://github.com/appc/spec/blob/master/spec/aci.md#image-manifest-schema):

```json
...
{
...
    "app": {
        ...
        "ports": [
            {
                "name": "8080-tcp",
                "protocol": "tcp",
                "port": 8080,
                "count": 1,
                "socketActivated": true
            }
        ]
    }
}
```

Then you will need a pair of service and socket unit files. The socket unit file will have the same port you've set in the image manifest of your app and the service unit file will just run `rkt`:


```
# my-socket-activated-app.service
[Unit]
Description=My socket-activated app

[Service]
ExecStart=/usr/bin/rkt run myapp.com/my-socket-activated-app:v1.0
KillMode=mixed
```

```
# my-socket-activated-app.socket
[Unit]
Description=My socket-activated app's socket

[Socket]
ListenStream=8080
```

Finally you start the socket unit:

```
# systemctl start my-socket-activated-app.socket
# systemctl status my-socket-activated-app.socket
● my-socket-activated-app.socket - My socket-activated app's socket
   Loaded: loaded (/etc/systemd/system/my-socket-activated-app.socket; static; vendor preset: disabled)
   Active: active (listening) since Thu 2015-07-30 12:24:50 CEST; 2s ago
   Listen: [::]:8080 (Stream)

Jul 30 12:24:50 locke-work systemd[1]: Listening on My socket-activated app's socket.
Jul 30 12:24:50 locke-work systemd[1]: Starting My socket-activated app's socket.
```

From this point, whenever you get a connection to port 8080 your container will start and handle it transparently.


## Using (not only) systemd tools

Let us assume that from now on, the service from the simple example unit file is started on the host.

### machinectl list

By using `systemd-nspawn`, we have integration with `systemd-machined` for free. Note the machine name (under `MACHINE` header) - it will show up in snippets later too. And you will need it for `systemd-run -M` or for `machinectl login` commands.

```
MACHINE                          CONTAINER SERVICE
rkt-6d0d9608-a744-4333-be21-942145a97a5a container nspawn

1 machines listed.
```

## ps auxf

The snippet below taken from output of `ps auxf` shows several things:

1. `rkt` `exec`s stage1's `systemd-nspawn` instead of using `fork-exec` technique. That is why you can not see it in the snippet.
2. `systemd-nspawn` runs a typical boot sequence - it spawns `systemd`, which in turn spawns our desired service(s).
3. There can be also other services running, which may be `systemd`-specific, like `systemd-journald`.

```
USER       PID %CPU %MEM    VSZ   RSS TTY      STAT START   TIME COMMAND
root      7258  0.2  0.0  19680  2664 ?        Ss   12:38   0:02 stage1/rootfs/usr/lib/ld-linux-x86-64.so.2 stage1/rootfs/usr/bin/systemd-nspawn --boot --register=true --link-journal=try-guest --quiet --keep-unit --uuid=6d0d9608-a744-4333-be21-942145a97a5a --machine=rkt-6d0d9608-a744-4333-be21-942145a97a5a --directory=stage1/rootfs -- --default-standard-output=tty --log-target=null --log-level=warning --show-status=0
root      7275  0.0  0.0  27348  4316 ?        Ss   12:38   0:00  \_ /usr/lib/systemd/systemd --default-standard-output=tty --log-target=null --log-level=warning --show-status=0
root      7277  0.0  0.0  23832  6100 ?        Ss   12:38   0:00      \_ /usr/lib/systemd/systemd-journald
root      7343  0.3  0.0  10652  7332 ?        Ssl  12:38   0:04      \_ /etcd
```

## systemd-cgls

`systemd-cgls` shows cgroups created on the host system (everything between the toplevel `system.slice` and the inner `system.slice`). The inner `system.slice` is a cgroup in `stage1`. The snippet below shows only the relevant portion of `systemd-cgls`' output.

```
├─1 /usr/lib/systemd/systemd --switched-root --system --deserialize 21
├─system.slice
│ ├─etcd.service
│ │ ├─7258 stage1/rootfs/usr/lib/ld-linux-x86-64.so.2 stage1/rootfs/usr/bin/systemd-nspawn --boot --register=true --link-journal=try-guest --quiet --keep-unit --uuid=6d0d9608-a744-4333-be21-942145a97a5a --machine=rkt-6d0d9608-a744-4333-be21-942145a97a5a --directory=stage1/rootfs -- --default-standard-output=tty --log-target=null --log-level=warning --show-status=0
│ │ ├─7275 /usr/lib/systemd/systemd --default-standard-output=tty --log-target=null --log-level=warning --show-status=0
│ │ └─system.slice
│ │   ├─systemd-journald.service
│ │   │ └─7277 /usr/lib/systemd/systemd-journald
│ │   └─etcd.service
│ │     └─7343 /etcd
```

## systemd-cgls --all

To actually see all the cgroups, use `--all` flag. This will show two cgroups for mount in host's `system.slice` - one for `stage1` root filesystem and one for `stage2` root filesystem. Inside pod's `system.slice` (the inner one) there are more mount cgroups - mostly for standard `/dev` devices.

```
├─1 /usr/lib/systemd/systemd --switched-root --system --deserialize 21
├─system.slice
│ ├─var-lib-rkt-pods-run-6d0d9608\x2da744\x2d4333\x2dbe21\x2d942145a97a5a-stage1-rootfs.mount
│ ├─var-lib-rkt-pods-run-6d0d9608\x2da744\x2d4333\x2dbe21\x2d942145a97a5a-stage1-rootfs-opt-stage2-etcd-rootfs.mount
│ ├─etcd.service
│ │ ├─7258 stage1/rootfs/usr/lib/ld-linux-x86-64.so.2 stage1/rootfs/usr/bin/systemd-nspawn --boot --register=true --link-journal=try-guest --quiet --keep-unit --uuid=6d0d9608-a744-4333-be21-942145a97a5a --machine=rkt-6d0d9608-a744-4333-be21-942145a97a5a --directory=stage1/rootfs -- --default-standard-output=tty --log-target=null --log-level=warning --show-status=0
│ │ ├─7275 /usr/lib/systemd/systemd --default-standard-output=tty --log-target=null --log-level=warning --show-status=0
│ │ └─system.slice
│ │   ├─proc-sys-kernel-random-boot_id.mount
│ │   ├─opt-stage2-etcd-rootfs-dev-random.mount
│ │   ├─opt-stage2-etcd-rootfs-dev-net-tun.mount
│ │   ├─-.mount
│ │   ├─system-prepare\x2dapp.slice
│ │   ├─opt-stage2-etcd-rootfs-dev-pts.mount
│ │   ├─opt-stage2-etcd-rootfs-sys.mount
│ │   ├─tmp.mount
│ │   ├─opt-stage2-etcd-rootfs.mount
│ │   ├─systemd-journald.service
│ │   │ └─7277 /usr/lib/systemd/systemd-journald
│ │   ├─opt-stage2-etcd-rootfs-proc.mount
│ │   ├─opt-stage2-etcd-rootfs-dev-urandom.mount
│ │   ├─etcd.service
│ │   │ └─7343 /etcd
│ │   ├─opt-stage2-etcd-rootfs-dev-tty.mount
│ │   ├─opt-stage2-etcd-rootfs-dev-console.mount
│ │   ├─run-systemd-nspawn-incoming.mount
│ │   ├─opt-stage2-etcd-rootfs-dev-zero.mount
│ │   ├─exit-watcher.service
│ │   ├─opt-stage2-etcd-rootfs-dev-null.mount
│ │   ├─opt-stage2-etcd-rootfs-dev-full.mount
│ │   └─opt-stage2-etcd-rootfs-dev-shm.mount
```

## journalctl -M

To see the logs from your service, use `journalctl -M <machine-id>`. You can get machine id from `machinectl list`.

```
-- Logs begin at Fri 2015-07-17 12:38:27 CEST, end at Fri 2015-07-17 12:38:29 CEST. --
Jul 17 12:38:27 rkt-6d0d9608-a744-4333-be21-942145a97a5a systemd-journal[2]: Runtime journal is using 8.0M (max allowed 384.2M, trying to leave 576.3M free of 3.7G available → current limit 384.2M).
Jul 17 12:38:27 rkt-6d0d9608-a744-4333-be21-942145a97a5a systemd-journal[2]: Permanent journal is using 8.0M (max allowed 4.0G, trying to leave 4.0G free of 4.9G available → current limit 924.4M).
Jul 17 12:38:27 rkt-6d0d9608-a744-4333-be21-942145a97a5a systemd-journal[2]: Time spent on flushing to /var is 1.103ms for 2 entries.
Jul 17 12:38:27 rkt-6d0d9608-a744-4333-be21-942145a97a5a systemd-journal[2]: Journal started
Jul 17 12:38:27 rkt-6d0d9608-a744-4333-be21-942145a97a5a etcd[4]: 2015/07/17 10:38:27 etcd: no data-dir provided, using default data-dir ./default.etcd
Jul 17 12:38:27 rkt-6d0d9608-a744-4333-be21-942145a97a5a etcd[4]: 2015/07/17 10:38:27 etcd: listening for peers on http://localhost:2380
Jul 17 12:38:27 rkt-6d0d9608-a744-4333-be21-942145a97a5a etcd[4]: 2015/07/17 10:38:27 etcd: listening for peers on http://localhost:7001
Jul 17 12:38:27 rkt-6d0d9608-a744-4333-be21-942145a97a5a etcd[4]: 2015/07/17 10:38:27 etcd: listening for client requests on http://localhost:2379
Jul 17 12:38:27 rkt-6d0d9608-a744-4333-be21-942145a97a5a etcd[4]: 2015/07/17 10:38:27 etcd: listening for client requests on http://localhost:4001
Jul 17 12:38:27 rkt-6d0d9608-a744-4333-be21-942145a97a5a etcd[4]: 2015/07/17 10:38:27 etcdserver: datadir is valid for the 2.0.1 format
Jul 17 12:38:27 rkt-6d0d9608-a744-4333-be21-942145a97a5a etcd[4]: 2015/07/17 10:38:27 etcdserver: name = default
Jul 17 12:38:27 rkt-6d0d9608-a744-4333-be21-942145a97a5a etcd[4]: 2015/07/17 10:38:27 etcdserver: data dir = default.etcd
Jul 17 12:38:27 rkt-6d0d9608-a744-4333-be21-942145a97a5a etcd[4]: 2015/07/17 10:38:27 etcdserver: member dir = default.etcd/member
Jul 17 12:38:27 rkt-6d0d9608-a744-4333-be21-942145a97a5a etcd[4]: 2015/07/17 10:38:27 etcdserver: heartbeat = 100ms
Jul 17 12:38:27 rkt-6d0d9608-a744-4333-be21-942145a97a5a etcd[4]: 2015/07/17 10:38:27 etcdserver: election = 1000ms
Jul 17 12:38:27 rkt-6d0d9608-a744-4333-be21-942145a97a5a etcd[4]: 2015/07/17 10:38:27 etcdserver: snapshot count = 10000
Jul 17 12:38:27 rkt-6d0d9608-a744-4333-be21-942145a97a5a etcd[4]: 2015/07/17 10:38:27 etcdserver: advertise client URLs = http://localhost:2379,http://localhost:4001
Jul 17 12:38:27 rkt-6d0d9608-a744-4333-be21-942145a97a5a etcd[4]: 2015/07/17 10:38:27 etcdserver: initial advertise peer URLs = http://localhost:2380,http://localhost:7001
Jul 17 12:38:27 rkt-6d0d9608-a744-4333-be21-942145a97a5a etcd[4]: 2015/07/17 10:38:27 etcdserver: initial cluster = default=http://localhost:2380,default=http://localhost:7001
Jul 17 12:38:28 rkt-6d0d9608-a744-4333-be21-942145a97a5a etcd[4]: 2015/07/17 10:38:28 etcdserver: start member ce2a822cea30bfca in cluster 7e27652122e8b2ae
Jul 17 12:38:28 rkt-6d0d9608-a744-4333-be21-942145a97a5a etcd[4]: 2015/07/17 10:38:28 raft: ce2a822cea30bfca became follower at term 0
Jul 17 12:38:28 rkt-6d0d9608-a744-4333-be21-942145a97a5a etcd[4]: 2015/07/17 10:38:28 raft: newRaft ce2a822cea30bfca [peers: [], term: 0, commit: 0, applied: 0, lastindex: 0, lastterm: 0]
Jul 17 12:38:28 rkt-6d0d9608-a744-4333-be21-942145a97a5a etcd[4]: 2015/07/17 10:38:28 raft: ce2a822cea30bfca became follower at term 1
Jul 17 12:38:28 rkt-6d0d9608-a744-4333-be21-942145a97a5a etcd[4]: 2015/07/17 10:38:28 etcdserver: added local member ce2a822cea30bfca [http://localhost:2380 http://localhost:7001] to cluster 7e27652122e8b2ae
Jul 17 12:38:29 rkt-6d0d9608-a744-4333-be21-942145a97a5a etcd[4]: 2015/07/17 10:38:29 raft: ce2a822cea30bfca is starting a new election at term 1
Jul 17 12:38:29 rkt-6d0d9608-a744-4333-be21-942145a97a5a etcd[4]: 2015/07/17 10:38:29 raft: ce2a822cea30bfca became candidate at term 2
Jul 17 12:38:29 rkt-6d0d9608-a744-4333-be21-942145a97a5a etcd[4]: 2015/07/17 10:38:29 raft: ce2a822cea30bfca received vote from ce2a822cea30bfca at term 2
Jul 17 12:38:29 rkt-6d0d9608-a744-4333-be21-942145a97a5a etcd[4]: 2015/07/17 10:38:29 raft: ce2a822cea30bfca became leader at term 2
Jul 17 12:38:29 rkt-6d0d9608-a744-4333-be21-942145a97a5a etcd[4]: 2015/07/17 10:38:29 raft.node: ce2a822cea30bfca elected leader ce2a822cea30bfca at term 2
Jul 17 12:38:29 rkt-6d0d9608-a744-4333-be21-942145a97a5a etcd[4]: 2015/07/17 10:38:29 etcdserver: published {Name:default ClientURLs:[http://localhost:2379 http://localhost:4001]} to cluster 7e27652122e8b2ae
```

## machinectl login

**WARNING**: This feature does not work at the moment.

**TODO**: Extend this documentation with output snippets and remove this TODO and the WARNING when dbus and required tools (like agetty and login) are also in stage1.

To login to a pod, use `machinectl login <machine-id>`. You can get the machine id from `machinectl list`. Note that `stage1` may not have all the tools you are used to (not even `ls`). It may not even have a shell, so in this case logging in to the pod is impossible.

## systemd-run -M

**WARNING**: This feature does not work at the moment.

**TODO**: Extend this documentation with output snippets and remove this TODO and the WARNING when dbus is also in stage1.

To run a program inside a pod, use `systemd-run -M <machine-id> <program-and-args>`. Note that `program` must exist inside `stage1`.
