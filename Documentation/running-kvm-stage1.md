# Running rkt with KVM stage1

rkt has support for executing pods with KVM hypervisor - [LKVM][lkvm] or [QEMU][qemu] as a [stage1][rkt-arch-stage1]. rkt employs this [alternative stage1][stage1-implementers-guide] to run a pod within a virtual machine with its own operating system kernel and hypervisor isolation, rather than creating a container using Linux cgroups and namespaces.

The KVM stage1 does not yet implement all of the default stage1's features and semantics. While the same app container can be executed under isolation by either stage1, it may require different configuration, especially for networking. However, several deployments of the KVM stage1 are operational outside of CoreOS, and we encourage testing of this feature and welcome your contributions.

## Getting started

Provided you have hardware virtualization support and the [kernel KVM module][kvm-module] loaded (refer to your distribution for instructions), you can then run an image like you would normally do with rkt:

```
sudo rkt run --debug --insecure-options=image --stage1-name=coreos.com/rkt/stage1-kvm:1.30.0 docker://redis
```

This output is the same you'll get if you run a container-based rkt.
If you want to see the kernel and boot messages, run rkt with the `--debug` flag.

You can exit pressing `<Ctrl-a x>`.

#### CPU usage
By default, processes will start working on all CPUs if at least one app does not have specified CPUs.
In the other case, container will be working on aggregate amount of CPUs.

#### Memory
Currently, the memory allocated to the virtual machine is a sum of memory required by each app in pod and additional 128MB required by system. If memory of some app is not specified, app memory will be set on default value (128MB).

## How does it work?

It leverages the work done by Intel with their [Clear Containers system][clear-containers].
Stage1 contains a Linux kernel that is executed under hypervisor (LKVM or QEMU).
This kernel will then start systemd, which in turn will start the applications in the pod.

A KVM-based rkt is very similar to a container-based one, it just uses hypervisor to execute pods instead of systemd-nspawn.

Here's a comparison of the components involved between a container-based and a KVM based rkt.

Container-based:

```
host OS
  └─ rkt
    └─ systemd-nspawn
      └─ systemd
        └─ chroot
          └─ user-app1
```


KVM based:

```
host OS
  └─ rkt
    └─ hypervisor
      └─ kernel
        └─ systemd
          └─ chroot
            └─ user-app1
```

## Building rkt KVM stage1

For LKVM you can use `stage1-kvm.aci` or `stage1-kvm-lkvm.aci`, for QEMU - `stage1-kvm-qemu.aci` from the official release. You can also build rkt yourself with the right options:

```
$ ./autogen.sh && ./configure --with-stage1-flavors=kvm --with-stage1-kvm-hypervisors=lkvm,qemu && make
```

For more details about configure parameters, see [configure script parameters documentation][build-configure].
This will build the rkt binary and the KVM stage1 aci image in `build-rkt-1.30.0+git/target/bin/`. Depending on the configuration options, it will be `stage1-kvm.aci` (if one hypervisor is set), or `stage1-kvm-lkvm.aci` and `stage1-kvm-qemu.aci` (if you want to have both images built once).


[build-configure]: build-configure.md
[clear-containers]: https://lwn.net/Articles/644675/
[kvm-module]: http://www.linux-kvm.org/page/Getting_the_kvm_kernel_modules
[lkvm]: https://kernel.googlesource.com/pub/scm/linux/kernel/git/will/kvmtool/+/master/README
[qemu]: http://qemu-project.org/Main_Page
[rkt-arch-stage1]: devel/architecture.md#stage-1
[rkt-run]: subcommands/run.md#use-a-custom-stage-1
[stage1-implementers-guide]: devel/stage1-implementors-guide.md

## Additional parameters

The KVM stage1 has some hypervisor specific parameters that can change the execution environment.

### Extra kernel command line parameters

Additional [Linux kernel's command line parameters](https://www.kernel.org/doc/html/latest/admin-guide/kernel-parameters.html) can be passed via the environment variable `RKT_HYPERVISOR_EXTRA_KERNEL_PARAMS`:

```
sudo RKT_HYPERVISOR_EXTRA_KERNEL_PARAMS="systemd.unified_cgroup_hierarchy=true max_loop=12 possible_cpus=1" \
      rkt run --stage1-name=coreos.com/rkt/stage1-kvm:1.30.0 \
      ...
```

The three command line parameters above are just examples and they are documented respectively in:
- [systemd's kernel command line parameters](https://www.freedesktop.org/software/systemd/man/kernel-command-line.html)
- [Linux' parameters](https://github.com/torvalds/linux/blob/master/Documentation/admin-guide/kernel-parameters.txt)
- [Linux' CPU hotplug](https://github.com/torvalds/linux/blob/master/Documentation/core-api/cpu_hotplug.rst)
