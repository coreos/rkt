# rkt status

Given a pod UUID, you can get the exit status of its apps.
Note that the apps are prefixed by `app-`.

```
$ rkt status 66ceb509
state=exited
created=2016-01-26 14:23:34.631 +0100 CET
started=2016-01-26 14:23:34.744 +0100 CET
pid=16964
exited=true
app-redis=0
app-etcd=0
```

If the pod is still running, you can wait for it to finish and then get the status with `rkt status --wait UUID`

## Options

| Flag | Default | Options | Description |
| --- | --- | --- | --- |
| `--wait` |  `false` | `true` or `false` | Toggle waiting for the pod to exit |

## Global options

See the table with [global options in general commands documentation](../commands.md#global-options).
