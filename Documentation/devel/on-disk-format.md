On disk format
==============

The data directory is normally `/var/lib/rkt` but rkt can be requested to use
another directory via the `--data` command line argument.

#### CAS database

The CAS database is stored in `/var/lib/rkt/cas/db`. The database schema can be
migrated to newer versions ([#706](https://github.com/coreos/rkt/issues/706)).

#### CAS

The CAS also uses other directories in `/var/lib/rkt/cas/`. To ensure stability
for the CAS, we need to make sure we don't remove any of those directories or
make any destructive changes to them. In future versions of rkt, we will make
sure to be able to read old versions of the CAS.

#### Pods

The pods are stored in `/var/lib/rkt/pods/` as explained in
[Life-cycle of a pod](https://github.com/coreos/rkt/blob/master/Documentation/devel/pod-lifecycle.md)

Stability of prepared and exited containers is desirable but not as critical as
the CAS.

#### Configuration

The [configuration](https://github.com/coreos/rkt/blob/master/Documentation/configuration.md)
on-disk format is documented separately.
