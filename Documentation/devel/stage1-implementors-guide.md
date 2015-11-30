Stage1 ACI implementor's guide
=============================

Background
----------

rkt's execution of pods is divided roughly into three separate stages:

0. Stage 0: discovering, fetching, verifying, storing, and compositing of both application (stage 2) and stage 1 images for execution.
1. Stage 1: execution of the stage 1 image from within the composite image prepared by stage 0.
2. Stage 2: execution of individual application images within the containment afforded by stage 1.

This separation of concerns is reflected in the file-system and layout of the composite image prepared by stage 0:

0. Stage 0: `rkt` executable, and the Pod Manifest created at "/var/lib/rkt/pods/$uuid/pod".
1. Stage 1: "stage1.aci", made available at "/var/lib/rkt/pods/$uuid/stage1" by `rkt run`.
2. Stage 2: "$app.aci", made available at "/var/lib/rkt/pods/$uuid/stage1/rootfs/opt/stage2/$appname" by `rkt run`, where $appname is the name of the app in the pod manifest.

The stage 1 implementation is what creates the execution environment for the contained applications.  This occurs via entrypoints from stage 0 on behalf of `rkt run` and `rkt enter`.  These entrypoints are nothing more than executable programs located via Annotations from within the stage 1 ACI manifest, and executed from within the stage 1 of a given pod at "/var/lib/rkt/pods/$uuid/stage1/rootfs".

Stage 2 is the destination application images and stage 1 is the vehicle for getting us there from stage 0.  For any given pod instance, the stage 1 may be completely different, allowing for flexibility in containment strategies employed within the same host while utilizing reusable application ACIs.

Entrypoints
-----------

### `rkt run` => "coreos.com/rkt/stage1/run"

0. rkt prepares the pod's stage 1 and stage 2 images and Pod Manifest under "/var/lib/rkt/pods/$uuid", acquiring an exclusive advisory lock on the directory.
1. chdirs to "/var/lib/rkt/pods/$uuid"
2. resolves the "coreos.com/rkt/stage1/run" entrypoint via Annotations found within "/var/lib/rkt/pods/$uuid/stage1/manifest"
3. executes the resolved entrypoint relative to "/var/lib/rkt/pods/$uuid/stage1/rootfs"

It is the responsibility of this entrypoint to consume the Pod Manifest and execute the constituent apps in the appropriate environments as specified by the Pod Manifest.

The environment variable "RKT_LOCK_FD" contains the file descriptor number of the open directory handle for "/var/lib/rkt/pods/$uuid".  It is necessary that stage 1 leave this file descriptor open and in its locked state for the duration of the `rkt run`.

In the bundled rkt stage 1 which includes systemd-nspawn and systemd, the entrypoint is a static Go program found at "/init" within the stage 1 ACI rootfs.  The majority of its execution entails generating a systemd-nspawn argument list and writing systemd unit files for the constituent apps before executing systemd-nspawn.  Systemd-nspawn then boots the stage 1 systemd with the just-written unit files for launching the contained apps.  The "/init" program is essentially a Pod Manifest to systemd-nspawn + systemd.service translator.

An alternative stage 1 could forego systemd-nspawn and systemd altogether, or retain these and introduce something like novm or qemu-kvm for greater isolation by first starting a VM.  All that is required is an executable at the place indicated by the "coreos.com/rkt/stage1/run" entrypoint which knows how to apply the Pod Manifest and prepared ACI file-systems to good effect.

The resolved entrypoint must inform rkt of its PID for the benefit of `rkt enter`. rkt supports two ways of doing that, stage1 implementors must pick one:

1. Writing to "/var/lib/rkt/pods/$uuid/pid" the PID of the process that is PID 1 in the container.
2. Writing to "/var/lib/rkt/pods/$uuid/ppid" the PID of the parent of the process that is PID 1 in the container. This assumes it has only one child.

#### Arguments
* `--debug` to activate debugging
* `--net[=$NET1,$NET2,...]` to configure the creation of a contained network. Please take the [network documentation](../networking.md) into account
* `--mds-token=$TOKEN` passes the auth token to the apps via `AC_METADATA_URL` env var
* `--interactive` to run a pod interactively, that is, pass standard input to the application (only for pods with one application)
* `--local-config=$PATH` to override the local configuration directory

### `rkt enter` => "coreos.com/rkt/stage1/enter"

0. rkt verifies the pod and image to enter are valid and running
1. chdirs to "/var/lib/rkt/pods/$uuid"
2. resolves the "coreos.com/rkt/stage1/enter" entrypoint via Annotations found within "/var/lib/rkt/pods/$uuid/stage1/manifest"
3. executes the resolved entrypoint relative to "/var/lib/rkt/pods/$uuid/stage1/rootfs"

In the bundled rkt stage 1 the entrypoint is a statically-linked C program found at "/enter" within the stage 1 ACI rootfs.  This program enters the namespaces of the systemd-nspawn container's PID 1 before executing the "/appexec" program which then chroots into the specific application's rootfs loading the application's environment variables as well.

An alternative stage 1 would need to do whatever is appropriate for entering the application's environment created by its own "coreos.com/rkt/stage1/run" entrypoint.

#### Arguments

1. `--pid=$PID` passes the PID of the process that is PID 1 in the container. rkt finds that PID by one of the two supported methods described in the `rkt run` section.
2. `--appname=$NAME` passes the app name of the specific application to enter.
3. the separator `--`
4. cmd to execute.
5. optionally, any cmd arguments.

### `rkt gc` => "coreos.com/rkt/stage1/gc"

It deals with garbage collecting resources allocated by stage1. For example, it removes the network namespace of a pod.

#### Arguments

* `--debug` to activate debugging
* UUID of the pod

Examples
--------

### Stage1 ACI manifest

```json
{
    "acKind": "ImageManifest",
    "acVersion": "0.7.3",
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
        }
    ]
}
```
