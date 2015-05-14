# Using rkt with systemd

**work in progress**

This document describes how to use rkt with systemd.

## Overview

rkt is designed and intended to be used with init systems like [systemd](http://www.freedesktop.org/wiki/Software/systemd/).
Since rkt consists only of a simple CLI that directly executes processes and writes to stdout/stderr (i.e. it does not daemonize), the lifecycle of rkt pods can be directly managed by systemd.
Consequently, standard systemd idioms like `systemctl start` and `systemctl stop` work out of the box.

## Simple Unit File

The following is a simple example of a unit file using rkt to run an nginx image:

```
[Unit]
Description=nginx

[Service]
ExecStart=/usr/bin/rkt --insecure-skip-verify run docker://nginx
Restart=always
```

This unit can now be managed using the standard `systemctl` commands:

```
systemctl start nginx.service
systemctl stop nginx.service
systemctl restart nginx.service
```

## Advanced Unit File

A more advanced unit example takes advantage of a few convenient systemd features:

1. Inheriting environment variables specified in the unit with `--inherit-env`. This functionality keeps your units clear and concise instead of layering on a ton of flags to `rkt run`.
2. Using the dependency graph to start our pod after networking has come online. This is helpful if your application requires outside connectivity to fetch remote configuration (for example, from etcd).
3. Set resource limits for this rkt pod. This can also be done in the unit instead of `rkt run`.
 
Here's what it looks like all together:

```
[Unit]
# Metadata
Description=MyApp
Documentation=https://myapp.com/docs/1.3.4
# Wait for networking
Requires=network-online.target
After=network-online.target

[Service]
# Resource limits
CPUShares=512
MemoryLimit=1G
# Env vars
Environment=HTTP_PROXY=192.0.2.3:5000
Environment=STORAGE_PATH=/opt/myapp
Environment=TMPDIR=/var/tmp
# Start the app
ExecStart=/usr/bin/rkt run --inherit-env --private-net --port=http:8888 myapp.com/myapp-1.3.4
Restart=always
```
