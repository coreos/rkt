# Installing rkt on popular Linux distributions

## CoreOS

rkt is an integral part of CoreOS, installed with the operating system.
The [CoreOS releases page](https://coreos.com/releases/) lists the version of rkt available in each CoreOS release channel.

If the version of rkt included in CoreOS is too old, it's fairly trivial to fetch the desired version [via a systemd unit](install-rkt-in-coreos.md)

## Debian

rkt currently is packaged in Debian sid (unstable) available at https://packages.debian.org/sid/utils/rkt:

```
sudo apt-get install rkt
```

Note that due to an outstanding bug (https://bugs.debian.org/cgi-bin/bugreport.cgi?bug=823322) one has to use the "coreos" stage1 image:

```
sudo rkt run --insecure-options=image --stage1-name=coreos.com/rkt/stage1-coreos:1.5.1 docker://nginx
```

If you prefer to install the newest version of rkt follow the instructions in the Ubuntu section below.

## Ubuntu

rkt is not packaged currently in Ubuntu. If you want the newest version of rkt the easiest method to install it is using the `install-rkt.sh` script:

```
sudo ./scripts/install-rkt.sh
```

The above script will install the "gnupg2", and "checkinstall" packages, download rkt, verify it, and finally invoke "checkinstall" to create a deb pakage and install rkt. To uninstall rkt, execute:

```
sudo apt-get remove rkt
```

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


## openSUSE

rkt is available in the [Virtualization:containers](https://build.opensuse.org/package/show/Virtualization:containers/rkt) project on openSUSE Build Service.
Before installing, the appropriate repository needs to be added (usually Tumbleweed or Leap):

```
sudo zypper ar -f obs://Virtualization:containers/openSUSE_Tumbleweed/ virtualization_containers
sudo zypper ar -f obs://Virtualization:containers/openSUSE_Leap_42.1/ virtualization_containers
```

Install rkt using zypper:

```
sudo zypper in rkt
```

## Void

rkt is available in the [official binary packages](http://www.voidlinux.eu/packages/) for the Void Linux distribution.
The source for these packages is hosted on [GitHub](https://github.com/voidlinux/void-packages/tree/master/srcpkgs/rkt).
