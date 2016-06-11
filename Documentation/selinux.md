# rkt and SELinux

rkt supports running containers using SELinux [SVirt](http://selinuxproject.org/page/SVirt).
The SELinux context for each rkt pod can be set in one of two ways:

### Reading SELinux context from the host lxc_contexts file

When invoking `rkt run` or `rkt run-prepared`, rkt will attempt to read `/etc/selinux/(policy)/contexts/lxc_contexts`.
If this file does not exist, no SELinux transitions will be performed.
Otherwise, rkt will generate a per-pod context.
All mounts for the pod will be created using the file context defined in `lxc_contexts`, and the processes inside the pod will be run in a context derived from the process context defined in `lxc_contexts`.

### Pass SELinux options
If `--selinux-options` is provided to `rkt run` or `rkt run-prepared`, then the given SELinux context will be used.
The flag is a comma separated string in the form of `user:USER,role:ROLE,type:TYPE,level:LEVEL`.
Users may specify a subset of the options, e.g. `--selinux-options=user:foo`.
The context fields defined in `--selinux-options` will override the fields in the context defined in `lxc_contexts`. If `--selinux-options=user:foo` is provided, then the `user` field will be overwritten as `foo`, other fields are not changed.

All processes running inside the pod will be created using the provided context.

In both ways mentioned above, the processes started in these contexts will be unable to interact with processes or files in any other pod's context, even though they are running as the same user.
Individual Linux distributions may impose additional isolation constraints on these contexts - please refer to your distribution documentation for further details.
