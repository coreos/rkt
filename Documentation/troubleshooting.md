# Troubleshooting

This document lists common rkt problems and how to fix or work around them.

## Missing container logs

When checking the logs of a container, they might be missing with an error like this:

```
$ journalctl -M rkt-3f045be0-1632-42f1-ba15-df984a82636f
Journal file /var/lib/rkt/pods/run/3f045be0-1632-42f1-ba15-df984a82636f/stage1/rootfs/var/log/journal/3f045be0163242f1ba15df984a82636f/system.journal uses an unsupported feature, ignoring file.
-- No entries --
```

This is because rkt's journald integration is only supported if systemd is compiled with `lz4` compression enabled.

You can check if it is enabled by making sure you see `+LZ4` when running `systemctl --version`:

```
$ systemctl --version
systemd 235
[...] +LZ4 [...]
```

## Bad system call

During rkt execution, you might encounter the message `Bad system call` followed by rkt terminating.
It's most likely a result of a too restrictive seccomp profile.

As a workaround, you can disable seccomp with `--insecure-options=seccomp`.

As a proper fix, you can [tweak the seccomp profile][seccomp-guide].

## Operation not permitted errors

During rkt execution, you might encounter a `Operation not permitted` message followed by rkt exiting.
Your image probably uses more capabilities than allowed in rkt's default list.

As a workaround, you can disable capabilities enforcement with `--insecure-options=capabilities`.

As a proper fix, you can [create your own list][capabilities-guide].

## BTRFS + overlay

```
prepare-app@opt-stage2-alpine\x2dsh-rootfs.service: Job prepare-app@opt-stage2-alpine\x2dsh-rootfs.service/start failed with result 'dependency'.
systemd-journald.service: Unit entered failed state.
systemd-journald.service: Failed with result 'signal'.
systemd-journald.service: Service has no hold-off time, scheduling restart.
```

To solve this update to Linux 4.5.2 or newer (see [#2175](https://github.com/rkt/rkt/issues/2175)).

## SELinux + overlay

You might se an error like this one when starting a rkt pod:

```
/usr/lib/systemd/systemd: error while loading shared libraries: libselinux.so.1: cannot open shared object file: Permission denied
```

The overlay filesystem doesn't work with SELinux in kernels older than 4.9 (see [1727](https://github.com/rkt/rkt/issues/1727)).
Please update your kernel to a newer version.

## Garbage collect not working in old kernels

You might see messages like these when running `rkt gc`:

```
Unable to remove pod "42e78965-c60b-4f4f-b412-484cd381fe90": remove /var/lib/rkt/pods/exited-garbage/42e78965-c60b-4f4f-b412-484cd381fe90/stage1/rootfs: device or resource busy
```

This might be due to using a kernel older than 3.18 (see [lazy umounts on unlinked files and directories](https://github.com/torvalds/linux/commit/8ed936b) and [#1922](https://github.com/rkt/rkt/issues/1922)).
Please update your kernel to a newer version.

## Running rkt on top of an overlay filesystem

Due to limitations in the Linux kernel, using rkt's overlay support on top of an overlay filesystem requires the upperdir and workdir to support the creation of trusted.* extended attributes and valid d_type in readdir responses (see [kernel/Documentation/filesystems/overlayfs.txt](https://www.kernel.org/doc/Documentation/filesystems/overlayfs.txt)).

The symptom is an error message like this:

```
stage0: error setting up stage1
  └─error rendering overlay filesystem
    └─problem mounting overlay filesystem
      └─error mounting overlay with options 'lowerdir=/var/lib/rkt/cas/tree/deps-sha512-f3d5f69d7faba1be7067d610f33131c18ac59eb43b1495016ade65bd13912578/rootfs,upperdir=/var/lib/rkt/pods/run/307bd207-7eab-4028-8d12-2d525e5b8ed9/overlay/deps-sha512-f3d5f69d7faba1be7067d610f33131c18ac59eb43b1495016ade65bd13912578/upper,workdir=/var/lib/rkt/pods/run/307bd207-7eab-4028-8d12-2d525e5b8ed9/overlay/deps-sha512-f3d5f69d7faba1be7067d610f33131c18ac59eb43b1495016ade65bd13912578/work' and dest '/var/lib/rkt/pods/run/307bd207-7eab-4028-8d12-2d525e5b8ed9/stage1/rootfs'
        └─invalid argument
```

This problem typically happens when trying to run rkt inside rkt.
To successfully run rkt inside rkt, use one of the following workarounds:
- set up `/var/lib/rkt` in the outer rkt as a host volume
- use `--no-overlay` for either the outer or the inner rkt

[capabilities-guide]: capabilities-guide.md
[seccomp-guide]: seccomp-guide.md
