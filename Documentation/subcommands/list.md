# rkt list

You can list all rkt pods.

```
# rkt list
UUID        APP     IMAGE NAME               STATE      NETWORKS
5bc080ca    redis   redis                    running    default:ip4=172.16.28.7
            etcd    coreos.com/etcd:v2.0.9
3089337c    nginx   nginx                    exited
```

You can view the full UUID as well as the image's ID by using the `--full` flag

```
# rkt list --full
UUID                                   APP     IMAGE NAME              IMAGE ID              STATE      NETWORKS
5bc080cav-9e03-480d-b705-5928af396cc5  redis   redis                   sha512-91e98d7f1679   running    default:ip4=172.16.28.7
                                       etcd    coreos.com/etcd:v2.0.9  sha512-a03f6bad952b
3089337c4-8021-119b-5ea0-879a7c694de4  nginx   nginx                   sha512-32ad6892f21a   exited
```
