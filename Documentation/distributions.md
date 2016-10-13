# Installing rkt on popular Linux distributions

- [Arch](#arch)
- [CentOS](#centos)
- [CoreOS](#coreos)
- [Debian](#debian)
- [Fedora](#fedora)
- [NixOS](#nixos)
- [openSUSE](#opensuse)
- [Ubuntu](#ubuntu)
- [Void](#void)

## rkt-maintained packages (manual installation)
- [rpm-based](#rpm-based)
- [deb-based](#deb-based)


## Distro-maintained packages
If your distribution packages rkt, then you should generally use their version. However,
if you need a newer version, you may choose to manually install the rkt-provided rpm and deb packages.

## Arch

rkt is available in the [Community Repository](https://www.archlinux.org/packages/community/x86_64/rkt/) and can be installed using pacman:
```
sudo pacman -S rkt
```

## CentOS

rkt is available in the [CentOS Community Build Service](https://cbs.centos.org/koji/packageinfo?packageID=4464) for CentOS 7.
However, this is [not yet ready for production use](https://github.com/coreos/rkt/issues/1305) due to pending systemd upgrade issues.

## CoreOS

rkt is an integral part of CoreOS, installed with the operating system.
The [CoreOS releases page](https://coreos.com/releases/) lists the version of rkt available in each CoreOS release channel.

If the version of rkt included in CoreOS is too old, it's fairly trivial to fetch the desired version [via a systemd unit](install-rkt-in-coreos.md).

## Debian

rkt is currently packaged in Debian sid (unstable) available at https://packages.debian.org/sid/utils/rkt:

```
sudo apt-get install rkt
```

Note that due to an outstanding bug (https://bugs.debian.org/cgi-bin/bugreport.cgi?bug=823322) one has to use the "coreos" stage1 image:

```
sudo rkt run --insecure-options=image --stage1-name=coreos.com/rkt/stage1-coreos:1.16.0 docker://nginx
```

If you don't run sid, or wish for a newer version, you can [install manually](#deb-based).

## Fedora

Fedora version 24 and up include rkt, with some caveats. __Careful!__ The version of rkt in Fedora
24 is very old. [Install manually](#rpm-based) instead. Fedora 25 and Rawhide [contain
recent versions](https://apps.fedoraproject.org/packages/rkt).

```
sudo dnf install rkt
```

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

## Ubuntu

rkt is not packaged currently in Ubuntu. Instead, install manually using the 
[rkt debian package](#deb-based).

## Void

rkt is available in the [official binary packages](http://www.voidlinux.eu/packages/) for the Void Linux distribution.
The source for these packages is hosted on [GitHub](https://github.com/voidlinux/void-packages/tree/master/srcpkgs/rkt).


# rkt-maintained packages
As part of the rkt build process, rpm and deb packages are built. If you need to use
the latest rkt version, or your distribution does not bundle rkt, these are available.

Currently rkt does not maintain an update server, so users of these packages must
upgrade manually.

### rpm-based 
```
gpg --recv-key 18AD5014C99EF7E3BA5F6CE950BDD3E0FC8A365E
wget https://github.com/coreos/rkt/releases/download/v1.16.0/rkt-1.16.0-1.x86_64.rpm
wget https://github.com/coreos/rkt/releases/download/v1.16.0/rkt-1.16.0-1.x86_64.rpm.asc
gpg --verify rkt-1.16.0-1.x86_64.rpm.asc
sudo rpm -Uvh rkt-1.16.0-1.x86_64.rpm
```

### deb-based
```
gpg --recv-key 18AD5014C99EF7E3BA5F6CE950BDD3E0FC8A365E
wget https://github.com/coreos/rkt/releases/download/v1.16.0/rkt_1.16.0-1_amd64.deb
wget https://github.com/coreos/rkt/releases/download/v1.16.0/rkt_1.16.0-1_amd64.deb.asc
gpg --verify rkt_1.16.0-1_amd64.deb.asc
sudo dpkg -i rkt_1.16.0-1_amd64.deb
```
