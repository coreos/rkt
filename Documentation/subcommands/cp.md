# rkt cp

Copy a file or directory from or to a running pod, given the pod UUID.

Files on the host can be referenced using absolute or relative paths. Files inside a pod are referenced using `UUID:ABSOLUTE_PATH`. Files can be copied from the host to a pod or the other way around. Copying files between pods is not supported. The destination path must reference an existing directory. File permissions are preserved on copy. Symlinks are not followed and skipped.

## Copy a file to a running pod

```
# rkt cp test.bin 76dc6286:/opt/
# rkt cp /usr/bin/test.bin 76dc6286:/opt/
```

## Copy a file from a running pod

```
# rkt cp 76dc6286:/opt/test.bin /tmp/
# rkt cp 76dc6286:/opt/test.bin .
```

## Specifying the app in the pod

```
# rkt cp test.bin 76dc6286:/opt/
Pod contains multiple apps:
        redis
        etcd
Unable to determine app name: specify app using "rkt cp --app= ..."

# rkt cp --app=redis test.bin 76dc6286:/opt/
```

## Paths containing colons

Files on the host with a filename that contains a colon can be referenced using an absolute path or a relative path starting with `./`.

```
# rkt cp /usr/bin/my:test.bin 76dc6286:/opt/
# rkt cp ./my:test.bin 76dc6286:/opt/
```

## Copying directories

Copying a directory copies all subdirectories and files. If the directory or subdirectories that do not exist yet will be created inside the destination directory. Files that already exist will be overwritten.

## Options

| Flag | Default | Options | Description |
| --- | --- | --- | --- |
| `--app` |  `` | Name of an app | Name of the app within the specified pod |

## Global options

See the table with [global options in general commands documentation][global-options].


[global-options]: ../commands.md#global-options
