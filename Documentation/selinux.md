# rkt and SELinux

rkt supports running containers using SELinux [SVirt][svirt].
At start-up, rkt will attempt to read `/etc/selinux/(policy)/contexts/lxc_contexts`.
If this file doesn't exist, no SELinux transitions will be performed.
If it does, rkt will generate a per-instance context.
All mounts for the instance will be created using the file context defined in `lxc_contexts`, and the instance processes will be run in a context derived from the process context defined in `lxc_contexts`.

Processes started in these contexts will be unable to interact with processes or files in any other instance's context, even though they are running as the same user.
Individual Linux distributions may impose additional isolation constraints on these contexts.

For example, given the following `lxc_contexts`:

```
# /etc/selinux/mcs/contexts/lxc_contexts
process = "system_u:system_r:svirt_lxc_net_t:s0"
content = "system_u:object_r:virt_var_lib_t:s0"
file = "system_u:object_r:svirt_lxc_file_t:s0"
```

You could define a policy where members of `svirt_lxc_net_t` context cannot write on TCP sockets.

Note that the policy is responsibility of your distribution and might differ from this example.

To find out more about policies you can check the [SELinux Policy Overview][selinux-policy].
Please refer to your distribution documentation for further details on its policy.

[svirt]: https://selinuxproject.org/page/SVirt
[selinux-policy]: https://www.centos.org/docs/5/html/Deployment_Guide-en-US/rhlcommon-chapter-0001.html

