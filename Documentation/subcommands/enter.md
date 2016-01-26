# rkt enter

Given a pod UUID, if you want to enter a running pod to explore its filesystem or see what's running you can use rkt enter.

```
# rkt enter 76dc6286
Pod contains multiple apps:
        redis
        etcd
Unable to determine app name: specify app using "rkt enter --app= ..."

# rkt enter --app=redis 76dc6286
No command specified, assuming "/bin/bash"
root@rkt-76dc6286-f672-45f2-908c-c36dcd663560:/# ls
bin   data  entrypoint.sh  home  lib64  mnt  proc  run   selinux  sys  usr
boot  dev   etc            lib   media  opt  root  sbin  srv      tmp  var
```

## Use a Custom Stage 1

rkt is designed and intended to be modular, using a [staged architecture](../devel/architecture.md).

You can use a custom stage1 by using the `--stage1-{url,path,name,hash,from-dir}` flags.

```
# rkt --stage1-path=/tmp/stage1.aci run coreos.com/etcd:v2.0.0
```

For more details see the [hacking documentation](../hacking.md).

## Run a Pod in the Background

Work in progress. Please contribute!
