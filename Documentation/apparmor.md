## AppArmor profiles

**Important**: this isolator is not part of the AppContainer spec, and is available for advanced users only.

Apps can be annoted to be confined by an AppArmor profile in a Pod manifest:

```
	"apps": [
		{
			"name": "myapp",
			"image": {
			  ...
			},
			"app": {
			  ...
			},
			"annotations": [
				{
					"name": "coreos.com/rkt/stage2/apparmor-profile",
					"value": "my-apparmor-profile"
				}
			]
		}
	],
```

Pods can be launched from manifests with `--pod-manifest=<path-to-json>`.

This requires support from the stage1 image being used, and will be a noop with stage1 images that don't support it.
The AppArmor profile being used must have been already loaded by the kernel or stage1 may error when trying to
confine the app.

Currently, none of the stage1 images bundled with rkt support this. Custom built `src` or `host` flavors using a
systemd compiled with AppArmor support will work.

### Disabling

use `--insecure-options=apparmor` to ignore AppArmor annotations.

### Verifying support

An example of what can be used to verify that apparmor labels are being properly applied by a stage1 image that
supports it:

```
$ cat manifest.json
{
        ...
        "apps": [
                {
                        "name": "myapp",
                        "image": {
                                ...
                        },
                        "app": {
                                "exec": [
                                        "/bin/bash"
                                ],
                                ...
                        },
                        "annotations": [
                                {
                                        "name": "coreos.com/rkt/stage2/apparmor-profile",
                                        "value": "docker-default"
                                }
                        ]
                }
        ]
        ...
}

$ sudo rkt --stage1-path=.../stage1-src.aci --pod-manifest=manifest.json --interactive
root@rkt-79f16dbc-7e72-49db-8281-68ca66fc886c:/# ps auxZ
LABEL                           USER       PID %CPU %MEM    VSZ   RSS TTY      STAT START   TIME COMMAND
unconfined                      root         1  0.6  0.0  42944  5980 ?        Ss   23:20   0:00 /usr/lib/systemd/systemd --default-standard-output=tty --log-target=null --show-s...
unconfined                      root        10  0.0  0.0  32052  7464 ?        Ss   23:20   0:00 /usr/lib/systemd/systemd-journald
docker-default (enforce)        root        12  1.0  0.0  18228  3208 console  Ss   23:20   0:00 /bin/bash
docker-default (enforce)        root        21  0.0  0.0  34424  2852 console  R+   23:20   0:00 ps auxZ
```

