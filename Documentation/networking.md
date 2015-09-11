# Networking

`rkt` has two networking options: "host" networking where the apps runs in the root host network namespace or "private" where the apps are allocated a new IP address and a new network namespace.

## Host (default) mode

By default, `rkt run` will start the pod with host networking.
This means that the apps within the pod will share the network stack and the interfaces with the host machine.

## Private networking mode

If `rkt run` is started with the `--private-net` flag, the pod will be executed with its own network stack, with the default network plus all configured networks.
This is equivalent to passing `--private-net=all`.
Passing a list of comma separated network names as in `--private-net=net1,net2,net3,...` restricts the network stack to the specified networks.
This can be useful for grouping certain pods together while separating others.

### The default network
The default network consists of a loopback device and a veth device.
The veth pair creates a point-to-point link between the pod and the host.
rkt will allocate an IPv4 /31 (2 IP addresses) out of 172.16.28.0/24 and assign one IP to each end of the veth pair.
It will additionally set the default route in the pod namespace.
Finally, it will enable IP masquerading on the host to NAT the egress traffic.

**Note**: The default network must be explicitly listed in order to be loaded when `--private-net=...` is specified with a list of network names.

Example: If you want default networking and two more networks you need to pass `--private-net=default,net1,net2`.

If *default* is not among the specified networks the *default-restricted* network will be added to the list of networks automatically.
The *default-restricted* network does not set up the default route and IP masquerading.
It only allows communication with the host via the veth interface and thus enables the pod to communicate with the metadata service which runs on the host.

### Setting up additional networks

In addition to the default network (veth) described in the previous section, rkt pods can be configured to join additional networks.
Each additional network will result in an new interface being setup in the pod.
The type of network interface, IP, routes, etc is controlled via a configuration file residing in `/etc/rkt/net.d` directory.
The network configuration files are executed in lexicographically sorted order. Each file consists of a JSON dictionary as shown below:

```sh
$ cat /etc/rkt/net.d/10-containers.conf
{
	"name": "containers",
	"type": "bridge",
	"ipam": {
		"type": "host-local",
		"subnet": "10.1.0.0/16"
	}
}
```

This configuration file defines a linux-bridge based network on 10.1.0.0/16 subnet.
The following fields apply to all configuration files.
Additional fields are specified for various types.

- **name** (string): An arbitrary label for the network. By convention the conf file is labeled with a leading ordinal, dash, network name, and .conf extension.
- **type** (string): The type of network/interface to create. The type actually names a network plugin. rkt is bundled with few built-in plugins.
- **ipam** (dict): IP Address Management -- controls the settings related to IP address assignment, gateway, and routes.

### Built-in network types

#### ptp

ptp is the probably the simplest type of networking and is used to set up default network. It creates a virtual ethernet pair (akin to a pipe) and places one end into pod and the other on the host. It is expected to be used with an IPAM type that will allocate a /31 for both ends of the veth (such as host-local-ptp). `ptp` specific configuration fields are:

- **mtu** (integer): the size of the MTU in bytes.
- **ipMasq** (boolean): whether to setup IP masquerading on the host.

#### bridge

Like the ptp type, `bridge` will also create a veth pair and place one end into the pod. However the host end of the veth will be plugged into a linux-bridge.
The configuration file specifies the bridge name and if the bridge does not exist, it will be created.
The bridge can optionally be setup to act as the gateway for the network. `bridge` specific configuration fields are:

- **bridge** (string): the name of the bridge to create and/or plug into. Defaults to `rkt0`.
- **isGateway** (boolean): whether the bridge should be assigned an IP and act as a gateway.
- **mtu** (integer): the size of the MTU in bytes for bridge and veths.
- **ipMasq** (boolean): whether to setup IP masquerading on the host.

#### macvlan

macvlan behaves similar to a bridge but does not provide communication between the host and the pod.

macvlan creates a virtual copy of a master interface and assigns the copy a randomly generated MAC address.
The pod can communicate with the network that is attached to the master interface.
The distinct MAC address allows the pod to be identified by external network services like DHCP servers, firewalls, routers, etc.
macvlan interfaces cannot communicate with the host via the macvlan interface.
This is because traffic that is sent by the pod onto the macvlan interface is bypassing the master interface and is sent directly to the interfaces underlying network.
Before traffic gets sent to the underlying network it can be evaluated within the macvlan driver, allowing it to communicate with all other pods that created their macvlan interface from the same master interface.

`macvlan` specific configuration fields are:
- **master** (string): the name of the host interface to copy. This field is required.
- **mode** (string): One of "bridge", "private", "vepa", or "passthru". This controls how traffic is handled between different macvlan interfaces on the same host. See [this guide](http://www.pocketnix.org/posts/Linux%20Networking:%20MAC%20VLANs%20and%20Virtual%20Ethernets) for discussion of modes. Defaults to "bridge".
- **mtu** (integer): the size of the MTU in bytes for the macvlan interface. Defaults to MTU of the master device.
- **ipMasq** (boolean): whether to setup IP masquerading on the host. Defaults to false.

#### ipvlan

ipvlan behaves very similar to macvlan but does not provide distinct MAC addresses for pods. 
macvlan and ipvlan can't be used on the same master device together.

ipvlan creates virtual copies of interfaces like macvlan but does not assign a new MAC address to the copied interface.
This does not allow the pods to be distinguished on a MAC level and so cannot be used with DHCP servers.
In other scenarios this can be an advantage, e.g. when an external network port does not allow multiple MAC addresses.
ipvlan also solves the problem of MAC address exhaustion that can occur with a large number of pods copying the same master interface.
ipvlan interfaces are able to have different IP addresses than the master interface and will therefore have the needed distinction for most use-cases.

`ipvlan` specific configuration fields are:
- **master** (string): the name of the host interface to copy. This field is required.
- **mode** (string): One of "l2", "l3". See [kernel documentation on ipvlan](https://www.kernel.org/doc/Documentation/networking/ipvlan.txt). Defaults to "l2".
- **mtu** (integer): the size of the MTU in bytes for the ipvlan interface. Defaults to MTU of the master device.
- **ipMasq** (boolean): whether to setup IP masquerading on the host. Defaults to false.

**Notes**
* ipvlan can cause problems with duplicated IPv6 link-local addresses since they
  are partially constructed using the MAC address. This issue is being currently
  [addressed by the ipvlan kernel module developers](http://thread.gmane.org/gmane.linux.network/363346/focus=363345)


## IP Address Management

The policy for IP address allocation, associated gateway and routes is separately configurable via the `ipam` section of the configuration file.
rkt currently ships with two IPAM types: host-local and DHCP. Like the network types, IPAM types can be implemented by third-parties via plugins.

### host-local

host-local type allocates IPs out of specified network range, much like a DHCP server would.
The difference is that while DHCP uses a central server, this type uses a static configuration.
Consider the following conf:

```sh
$ cat /etc/rkt/net.d/10-containers.conf
{
	"name": "containers",
	"type": "bridge",
	"bridge": "rkt1",
	"ipam": {
		"type": "host-local",
		"subnet": "10.1.0.0/16",
	}
}
```

This configuration instructs rkt to create `rkt1` Linux bridge and plugs pods into it via veths.
Since the subnet is defined as `10.1.0.0/16`, rkt will assign individual IPs out of that range.
The first pod will be assigned 10.1.0.2/16, next one 10.1.0.3/16, etc (it reserves 10.1.0.1/16 for gateway).
Additional configuration fields:

- **subnet** (string): Subnet in CIDR notation for the network.
- **rangeStart** (string): First IP address from which to start allocating IPs. Defaults to second IP in `subnet` range.
- **rangeEnd** (string): Last IP address in the allocatable range. Defaults to last IP in `subnet` range.
- **gateway** (string): The IP address of the gateway in this subnet.
- **routes** (list of strings): List of IP routes in CIDR notation. The routes get added to pod namespace with next-hop set to the gateway of the network.

The following shows a more complex IPv6 example in combination with the ipvlan plugin. The gateway is configured for the default
route, allowing the pod to access external networks via the ipvlan interface.

```json
{
    "name": "ipv6-public",
    "type": "ipvlan",
    "master": "em1",
    "mode": "l3",
    "ipam": {
        "type": "host-local",
        "subnet": "2001:0db8:161:8374::/64",
        "rangeStart": "2001:0db8:161:8374::1:2",
        "rangeEnd": "2001:0db8:161:8374::1:fffe",
        "gateway": "fe80::1",
        "routes": [
            { "dst": "::0/0" }
        ]
    }
}
```

### dhcp

DHCP type requires a daemon to be running on the host.
The DHCP plugin binary can be executed in the daemon mode by launching it with `daemon` argument.
However the DHCP plugin is bundled in stage1.aci so this requires extracting the binary from it:

```
$ sudo ./rkt fetch --insecure-skip-verify ./stage1.aci
$ sudo ./rkt image extract coreos.com/rkt/stage1 /tmp/stage1
$ sudo cp /tmp/stage1/rootfs/usr/lib/rkt/plugins/net/dhcp .
```

Now start the daemon:

```
$ sudo ./dhcp daemon
```

It is now possible to use the DHCP type by specifying it in the ipam section of the network configuration file:

```
{
	"name": "lan",
	"type": "macvlan",
	"master": "eth0",
	"ipam": {
		"type": "dhcp"
	}
}
```

For more information about DHCP plugin, see [CNI docs](https://github.com/appc/cni/blob/master/Documentation/dhcp.md).

## Other plugins

### flannel
This plugin is designed to work in conjunction with flannel, a network fabric for containers.
The basic network configuration is as follows:
```json
{
	"name": "containers",
	"type": "flannel"
}
```

This will setup a linux-bridge, connect the container to the bridge and assign container IPs out of the subnet that flannel assigned to the host.
For more information included advanced configuration options, see [CNI docs](https://github.com/appc/cni/blob/master/Documentation/flannel.md).

## Exposing container ports on the host
Apps declare their public ports in the image manifest file.
A user can expose some or all of these ports to the host when running a pod.
Doing so allows services inside the pods to be reachable through the host's IP address.

The example below demonstrates an image manifest snippet declaring a single port:

```
"ports": [
	{
		"name": "http",
		"port": 80,
		"protocol": "tcp"
	}
]
```

The pod's TCP port 80 can be mapped to an arbitrary port on the host during rkt invocation:

```
# rkt run --private-net --port=http:8888 myapp.aci
```

Now, any traffic arriving on host's TCP port 8888 will be forwarded to the pod on port 80.

### Overriding default network
If a network has a name "default", it will override the default network added
by rkt. It is strongly recommended that such network also has type "veth" as
it protects from the pod spoofing its IP address and defeating identity
management provided by the metadata service.

### Overriding network settings
The network backend CNI allows to pass [arguments as plugin parameters](https://github.com/appc/cni/blob/master/SPEC.md#parameters), specifically `CNI_ARGS`, at runtime.
These arguments can be used to reconfigure a network without changing the configuration file.
rkt supports the `CNI_ARGS` variable through the command line argument `--private-net-args`. 
The argument can be passed multiple times to allow passing arguments to different networks.

The syntax for one instance of the argument is `--private-net-args="$networkname:$arg1=$val1;$arg2=val2"`.
(Please note the mandatory usage of `"` due to the `;` being used as separator)


#### Supported CNI_ARGS
This is not documented yet, except for the example that follows a couple lines down.
Please follow [this issue on CNI](https://github.com/appc/cni/issues/56) to track the progress of the documentation.

#### Example: Override IPs for two different networks
This example would override the IP addresses for two different networks:
```bash
rkt run --private-net=net1,net2 --private-net-args="net1:IP=1.2.3.4" --private-net-args="net2:IP=1.2.4.5" pod.aci"
```
