# Installing rkt on popular Linux distributions

## CoreOS

rkt is an integral part of CoreOS, installed with the operating system.
The [CoreOS releases page](https://coreos.com/releases/) lists the version of rkt available in each CoreOS release channel.

## Fedora

rkt is packaged in the development version of Fedora, [Rawhide](https://fedoraproject.org/wiki/Releases/Rawhide):
```
sudo dnf install rkt
```

Until the rkt package makes its way into the general Fedora releases, [download the latest rkt directly from the project](https://github.com/coreos/rkt/releases).

rkt's entry in the [Fedora package database](https://admin.fedoraproject.org/pkgdb/package/rpms/rkt/) tracks packaging work for this distribution.

#### Caveat: SELinux

rkt can integrate with SELinux on Fedora but in a limited way.
This has the following caveats:
- running as systemd service restricted (see [#2322](https://github.com/coreos/rkt/issues/2322))
- access to host volumes restricted (see [#2325](https://github.com/coreos/rkt/issues/2325))
- socket activation restricted (see [#2326](https://github.com/coreos/rkt/issues/2326))
- metadata service restricted (see [#1978](https://github.com/coreos/rkt/issues/1978))

As a workaround, SELinux can be temporarily disabled:
```
sudo setenforce Permissive
```
Or permanently disabled by editing `/etc/selinux/config`:
```
SELINUX=permissive
```

#### Caveat: firewalld

Fedora uses [firewalld](https://fedoraproject.org/wiki/FirewallD) to dynamically define firewall zones.
rkt is [not yet fully integrated with firewalld](https://github.com/coreos/rkt/issues/2206).
The default firewalld rules may interfere with the network connectivity of rkt pods.
To work around this, add a firewalld rule to allow pod traffic:
```
sudo firewall-cmd --add-source=172.16.28.0/24 --zone=trusted
```

172.16.28.0/24 is the subnet of the [default pod network](https://github.com/coreos/rkt/blob/master/Documentation/networking/overview.md#the-default-network). The command must be adapted when rkt is configured to use a [different network](https://github.com/coreos/rkt/blob/master/Documentation/networking/overview.md#setting-up-additional-networks) with a different subnet.


## Arch

rkt is available in the [Community Repository](https://www.archlinux.org/packages/community/x86_64/rkt/) and can be installed using pacman:
```
sudo pacman -S rkt
```

## NixOS

rkt can be installed on NixOS using the following command:

```
nix-env -iA rkt
```

The source for the rkt.nix expression can be found on [GitHub](https://github.com/NixOS/nixpkgs/blob/master/pkgs/applications/virtualization/rkt/default.nix)

## Void

rkt is available in the [official binary packages](http://www.voidlinux.eu/packages/) for the Void Linux distribution.
The source for these packages is hosted on [GitHub](https://github.com/voidlinux/void-packages/tree/master/srcpkgs/rkt).
