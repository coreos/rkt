# Stage1 ACI implementor's guide

## Background

rkt's execution of pods is divided roughly into three separate stages:

1. Stage 0: discovering, fetching, verifying, storing, and compositing of both application (stage2) and stage1 images for execution.
2. Stage 1: execution of the stage1 image from within the composite image prepared by stage0.
3. Stage 2: execution of individual application images within the containment afforded by stage1.

This separation of concerns is reflected in the file-system and layout of the composite image prepared by stage0:

1. Stage 0: `rkt` executable, and the pod manifest created at `/var/lib/rkt/pods/prepare/$uuid/pod`.
2. Stage 1: `stage1.aci`, made available at `/var/lib/rkt/pods/run/$uuid/stage1` by `rkt run`.
3. Stage 2: `$app.aci`, made available at `/var/lib/rkt/pods/run/$uuid/stage1/rootfs/opt/stage2/$appname` by `rkt run`, where `$appname` is the name of the app in the pod manifest.

The stage1 implementation is what creates the execution environment for the contained applications.
This occurs via entrypoints from stage0 on behalf of `rkt run` and `rkt enter`.
These entrypoints are executable programs located via annotations from within the stage1 ACI manifest, and executed from within the stage1 of a given pod at `/var/lib/rkt/pods/$state/$uuid/stage1/rootfs`.

Stage2 is the deployed application image.
Stage1 is the vehicle for getting there from stage0.
For any given pod instance, stage1 may be replaced by a completely different implementation.
This allows users to employ different containment strategies on the same host running the same interchangeable ACIs.

## Entrypoints

### rkt run

`coreos.com/rkt/stage1/run`

1. rkt prepares the pod's stage1 and stage2 images and pod manifest under `/var/lib/rkt/pods/prepare/$uuid`, acquiring an exclusive advisory lock on the directory.
   Upon a successful preparation, the directory will be renamed to `/var/lib/rkt/pods/run/$uuid`.
2. chdirs to `/var/lib/rkt/pods/run/$uuid`.
3. resolves the `coreos.com/rkt/stage1/run` entrypoint via annotations found within `/var/lib/rkt/pods/run/$uuid/stage1/manifest`.
4. executes the resolved entrypoint relative to `/var/lib/rkt/pods/run/$uuid/stage1/rootfs`.

It is the responsibility of this entrypoint to consume the pod manifest and execute the constituent apps in the appropriate environments as specified by the pod manifest.

The environment variable `RKT_LOCK_FD` contains the file descriptor number of the open directory handle for `/var/lib/rkt/pods/run/$uuid`.
It is necessary that stage1 leave this file descriptor open and in its locked state for the duration of the `rkt run`.

In the bundled rkt stage1 which includes systemd-nspawn and systemd, the entrypoint is a static Go program found at `/init` within the stage1 ACI rootfs.
The majority of its execution entails generating a systemd-nspawn argument list and writing systemd unit files for the constituent apps before executing systemd-nspawn.
Systemd-nspawn then boots the stage1 systemd with the just-written unit files for launching the contained apps.
The `/init` program's primary job is translating a pod manifest to systemd-nspawn systemd.services.

An alternative stage1 could forego systemd-nspawn and systemd altogether, or retain these and introduce something like novm or qemu-kvm for greater isolation by first starting a VM.
All that is required is an executable at the place indicated by the `coreos.com/rkt/stage1/run` entrypoint that knows how to apply the pod manifest and prepared ACI file-systems.

The resolved entrypoint must inform rkt of its PID for the benefit of `rkt enter`.
Stage1 implementors have two options for doing so; only one must be implemented:

* `/var/lib/rkt/pods/run/$uuid/pid`: the PID of the process that will be given to the "enter" entrypoint.
* `/var/lib/rkt/pods/run/$uuid/ppid`: the PID of the parent of the process that will be given to the "enter" entrypoint. That parent process must have exactly one child process.

The entrypoint of a stage1 may also optionally inform rkt of the "pod cgroup", the `name=systemd` cgroup the pod's applications are expected to reside under, via the `subcgroup` file. If this file is written, it must be written before the `pid` or `ppid` files are written. This information is useful for any external monitoring system that wishes to reliably link a given cgroup to its associated rkt pod. The file should be written in the pod directory at `/var/lib/rkt/pods/run/$uuid/subcgroup`.

The file's contents should be a text string, for example of the form `machine-rkt\xuuid.scope`, which will match the control in the cgroup hierarchy of the `ppid` or `pid` of the pod.

Any stage1 that supports and expects machined registration to occur will likely want to write such a file.

#### Arguments

* `--debug` to activate debugging
* `--net[=$NET1,$NET2,...]` to configure the creation of a contained network.
  See the [rkt networking documentation](../networking/overview.md) for details.
* `--mds-token=$TOKEN` passes the auth token to the apps via `AC_METADATA_URL` env var
* `--interactive` to run a pod interactively, that is, pass standard input to the application (only for pods with one application)
* `--local-config=$PATH` to override the local configuration directory
* `--private-users=$SHIFT` to define a UID/GID shift when using user namespaces. SHIFT is a two-value colon-separated parameter, the first value is the host UID to assign to the container and the second one is the number of host UIDs to assign.
* `--mutable` activates a mutable environment in stage1. If the stage1 image manifest has no `app` entrypoint annotations declared, this flag will be unset to retain backwards compatibility.

#### Arguments added in interface version 2

* `--hostname=$HOSTNAME` configures the host name of the pod. If empty, it will be "rkt-$PODUUID".

#### Arguments added in interface version 3

* `--disable-capabilities-restriction` gives all capabilities to apps (overrides `retain-set` and `remove-set`)
* `--disable-paths` disables inaccessible and read-only paths (such as `/proc/sysrq-trigger`)
* `--disable-seccomp` disables seccomp (overrides `retain-set` and `remove-set`)

#### Arguments added in interface version 4

* `--dns-conf-mode=resolv=(host|stage0|none|default),hosts=(host|stage0|default)`: Configures how the stage1 should set up
	the DNS configuration files `/etc/resolv.conf` and `/etc/hosts`. For all, `host` means to bind-mount the host's
	version of that file. `none` means the stage1 should not create it. `stage0` means the stage0 has created an entry
	in the stage1's rootfs, which should be exposed in the apps. `default` means the standard behavior, which for 
	`resolv.conf` is to create /etc/rkt-resolv.conf iff a CNI plugin specifies it, and for `hosts` is to create 
	a fallback if the app does not provide it.

### rkt enter

`coreos.com/rkt/stage1/enter`

1. rkt verifies the pod and image to enter are valid and running
2. chdirs to `/var/lib/rkt/pods/run/$uuid`
3. resolves the `coreos.com/rkt/stage1/enter` entrypoint via annotations found within `/var/lib/rkt/pods/run/$uuid/stage1/manifest`
4. executes the resolved entrypoint relative to `/var/lib/rkt/pods/run/$uuid/stage1/rootfs`

In the bundled rkt stage1, the entrypoint is a statically-linked C program found at `/enter` within the stage1 ACI rootfs.
This program enters the namespaces of the systemd-nspawn container's PID 1 before executing the `/enterexec` program.
`enterexec` then `chroot`s into the ACI's rootfs, loading the application and its environment.

An alternative stage1 would need to do whatever is appropriate for entering the application environment created by its own `coreos.com/rkt/stage1/run` entrypoint.

#### Arguments

1. `--pid=$PID` passes the PID of the process that is PID 1 in the container.
   rkt finds that PID by one of the two supported methods described in the `rkt run` section.
2. `--appname=$NAME` passes the app name of the specific application to enter.
3. the separator `--`
4. cmd to execute.
5. optionally, any cmd arguments.

### rkt gc

`coreos.com/rkt/stage1/gc`

The gc entrypoint deals with garbage collecting resources allocated by stage1.
For example, it removes the network namespace of a pod.

#### Arguments

* `--debug` to activate debugging
* UUID of the pod

### rkt stop

`coreos.com/rkt/stage1/stop`

The optional stop entrypoint initiates an orderly shutdown of stage1.

In the bundled rkt stage 1, the entrypoint is sending SIGTERM signal to systemd-nspawn. For kvm flavor, it is calling `systemctl halt` on the container (through SSH).

#### Arguments

* `--force` to force the stopping of the pod. E.g. in the bundled rkt stage 1, stop sends SIGKILL
* UUID of the pod

### rkt app add

`coreos.com/rkt/stage1/app/add`

#### Arguments

`start $OPTIONS UUID APPNAME ENTERENTRYPOINT PID`

* `--debug` to activate debugging
* `--disable-capabilities-restriction` gives all capabilities to apps (overrides `retain-set` and `remove-set`)
* `--disable-paths` disables inaccessible and read-only paths (such as `/proc/sysrq-trigger`)
* `--disable-seccomp` disables seccomp (overrides `retain-set` and `remove-set`)
* `--private-users=$SHIFT` to define a UID/GID shift when using user namespaces. SHIFT is a two-value colon-separated parameter, the first value is the host UID to assign to the container and the second one is the number of host UIDs to assign.

### rkt app start

`coreos.com/rkt/stage1/app/start`

#### Arguments

`start $OPTIONS UUID APPNAME ENTERENTRYPOINT PID`

* `--debug` to activate debugging

### rkt app stop

`coreos.com/rkt/stage1/app/stop`

#### Arguments

`stop $OPTIONS UUID APPNAME ENTERENTRYPOINT PID`

* `--debug` to activate debugging

### rkt app rm

`coreos.com/rkt/stage1/app/rm`

#### Arguments

`rm $OPTIONS UUID APPNAME ENTERENTRYPOINT PID`

* `--debug` to activate debugging

## Metadata

### Mutable pods

Stage1 images can support mutable pod environments, where, once a pod has been started, applications can be added/started/stopped/removed while the actual pod is running. This information is persisted at runtime in the pod manifest using the `coreos.com/rkt/stage1/mutable` annotation.

If the annotation is not present, `false` is assumed.

### Versioning

The stage1 command line interface is versioned using an annotation with the name `coreos.com/rkt/stage1/interface-version`.
If the annotation is not present, rkt assumes the version is 1.

The current version of the stage1 interface is 3.

## Examples

### Stage1 ACI manifest

```json
{
    "acKind": "ImageManifest",
    "acVersion": "0.8.8",
    "name": "foo.com/rkt/stage1",
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
    "annotations": [
        {
            "name": "coreos.com/rkt/stage1/run",
            "value": "/ex/run"
        },
        {
            "name": "coreos.com/rkt/stage1/enter",
            "value": "/ex/enter"
        },
        {
            "name": "coreos.com/rkt/stage1/gc",
            "value": "/ex/gc"
        },
        {
            "name": "coreos.com/rkt/stage1/stop",
            "value": "/ex/stop"
        },
        {
            "name": "coreos.com/rkt/stage1/interface-version",
            "value": "2"
        }
    ]
}
```

## Filesystem Layout Assumptions

The following paths are reserved for the stage1 image, and they will be created during stage0.
When creating a stage1 image, developers SHOULD NOT create or use these paths in the image's filesystem.

### stage2

`opt/stage2`

This directory path is used for extracting the ACI of every app in the pod.
Each app's rootfs will appear under this directory,
e.g. `/var/lib/rkt/pods/run/$uuid/stage1/rootfs/opt/stage2/$appname/rootfs`.

### status

`rkt/status`

This directory path is used for storing the apps' exit statuses.
For example, if an app named `foo` exits with status = `42`, stage1 should write `42`
in `/var/lib/rkt/pods/run/$uuid/stage1/rootfs/rkt/status/foo`.
Later the exit status can be retrieved and shown by `rkt status $uuid`.

### env

`rkt/env`

This directory path is used for passing environment variables to each app.
For example, environment variables for an app named `foo` will be stored in `rkt/env/foo`.
