# rkt architecture

## Overview

rkt consists only of a command-line tool, `rkt`, and does not have a daemon. This architecture allows rkt to be updated in-place without affecting application containers which are currently running. It also means that levels of privilege can be separated out between different operations.

All state in rkt is communicated via the filesystem. Facilities like file-locking are used to ensure co-operation and mutual exclusion between concurrent invocations of the `rkt` command.

## Stages

Execution with rkt is divided into several distinct stages.

_**NB** The goal is for the ABI between stages to be relatively fixed, but while rkt is still under heavy development this is still evolving. Until https://github.com/coreos/rkt/issues/572 is resolved, this should be considered in flux and the description below may not be authoritative._

### Stage 0

The first stage is the actual `rkt` binary itself. When running a pod, this binary is responsible for performing a number of initial preparatory tasks:
- Fetching the specified ACIs, including the stage 1 ACI of --stage1-image if specified.
- Generating a Pod UUID
- Generating a Pod Manifest
- Creating a filesystem for the pod
- Setting up stage 1 and stage 2 directories in the filesystem
- Unpacking the stage 1 ACI into the pod filesystem
- Unpacking the ACIs and copying each app into the stage2 directories

Given a run command such as:

```
$ sudo rkt run \
	sha512-8a30f14877cd8065939e3912542a17d1a5fd9b4c \
	sha512-abcd29837d89389s9d0898ds908ds890df890908
```

a pod manifest compliant with the ACE spec will be generated, and the filesystem created by stage0 should be:

```
/pod
/stage1
/stage1/manifest
/stage1/rootfs/init
/stage1/rootfs/opt
/stage1/rootfs/opt/stage2/0
/stage1/rootfs/opt/stage2/1
```

where:
- `pod` is the pod manifest file
- `stage1` is a copy of the stage1 ACI that is safe for read/write
- `stage1/manifest` is the manifest of the stage1 ACI
- `stage1/rootfs` is the rootfs of the stage1 ACI
- `stage1/rootfs/init` is the actual stage1 binary to be executed (this path may vary according to the `coreos.com/rkt/stage1/init` Annotation of the stage1 ACI)
- `stage1/rootfs/opt/stage2` are copies of the unpacked ACIs

At this point the stage0 execs `/stage1/rootfs/init` with the current working directory set to the root of the new filesystem.

### Stage 1

The next stage is a binary that the user trusts to set up cgroups, execute processes, and perform other operations as root on the host. This stage has the responsibility of taking the execution group filesystem that was created by stage 0 and creating the necessary cgroups, namespaces and mounts to launch the execution group:

- Generate systemd unit files from the Image and Pod Manifests. The Image Manifest defines the default `exec` specifications of each application; the Pod Manifest defines the ordering of the units, as well as any `exec` overrides.
- (containing, respectively, the exec specifications of each pod and the ordering given by the user)
- Set up any external volumes (undefined at this point)
- nspawn attach to the bridge and launch the execution group systemd
- Launch the root systemd
- Have the root systemd

This process is slightly different for the qemu-kvm stage1 but a similar workflow starting at `exec()`'ing kvm instead of an nspawn.

### Stage 2

The final stage is executing the actual application. The responsibilities of the stage2 include:

- Launch the init process described in the Image Manifest
