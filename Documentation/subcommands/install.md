# rkt install

This command sets up the rkt data directory (`/var/lib/rkt` by default) with the right permissions, allowing users that belong to the `rkt` group to get information about images and pods without having to be root.

## Global options

| Flag | Default | Options | Description |
| --- | --- | --- | --- |
| `--debug` |  `false` | `true` or `false` | Prints out more debug information to `stderr` |
| `--dir` | `/var/lib/rkt` | A directory path | Path to the `rkt` data directory |
| `--insecure-options` |  none | <ul><li>**none**: All security features are enabled</li><li>**http**: Allow HTTP connections. Be warned that this will send any credentials as clear text.</li><li>**image**: Disables verifying image signatures</li><li>**tls**: Accept any certificate from the server and any host name in that certificate</li><li>**ondisk**: Disables verifying the integrity of the on-disk, rendered image before running. This significantly speeds up start time.</li><li>**all**: Disables all security checks</li></ul>  | Comma-separated list of security features to disable |
| `--local-config` |  `/etc/rkt` | A directory path | Path to the local configuration directory |
| `--system-config` |  `/usr/lib/rkt` | A directory path | Path to the system configuration directory |
| `--trust-keys-from-https` |  `false` | `true` or `false` | Automatically trust gpg keys fetched from https |
| `--user-config` |  `` | A directory path | Path to the user configuration directory |

## rkt install stage1

This installs given stage1 image in the store and generates a configuration file in a system configuration directory.

```
# rkt --insecure-options=image install stage1 /usr/lib/rkt/stage1-images/stage1-coreos.aci
```
