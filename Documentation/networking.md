# Networking

`rkt` has two networking options: "host" networking where the apps runs in the root host network namespace or "private" where the apps are allocated a new IP address and a new network namespace.

## Host (default) mode

By default, `rkt run` will start the pod with host networking.
This means that the apps within the pod will share the network stack and the interfaces with the host machine.
Since the metadata service uses the pod IP for identity, rkt will _not_ register the pod with the metadata service in this mode.

Note: because of the lack of the metadata service rkt does not strictly implement the app container specification in host mode.

## Private networking mode

For all of the private networking options the metadata service, launched via `rkt metadata-service`, must be running.
The service will listen on 0.0.0.0:2375 by default and provides the private networking containers the metadata services described in the App Container Spec.
Ideally this metadata service is launched via your systems init system.

If `rkt run` is started with `--private-net`, the pod will be executed with its own network stack.
By default, rkt will create a loopback device and a veth device. The veth pair creates a point-to-point link between the pod and the host.
rkt will allocate an IPv4 /31 (2 IP addresses) out of 172.16.28.0/24 and assign one IP to each end of the veth pair.
It will additionally set a route for metadata service (169.254.169.255/32) and default route in the pod namespace.
Finally, it will enable IP masquerading on the host to NAT the egress traffic.

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
- **type** (string): The type of network/interface to create. The type actually names a network plugin with rkt bundled with few built-in ones.
- **ipam** (dict): IP Address Management -- controls the settings related to IP address assignment, gateway, and routes.

### Built-in network types

#### veth

veth is the probably the simplest type of networking and is used to set up default network. It creates a virtual ethernet pair (akin to a pipe) and places one end into pod and the other on the host. It is expected to be used with IPAM type that will allocate a /31 for both ends of the veth (such as host-local-ptp). `veth` specific configuration fields are:

- **mtu** (integer): the size of the MTU in bytes.
- **ipMasq** (boolean): whether to setup IP masquerading on the host.

#### bridge

Like the veth type, `bridge` will also create a veth pair and place one end into the pod. However the host end of the veth will be plugged into a linux-bridge.
The configuration file specifies the bridge name and if the bridge does not exist, it will be created.
The bridge can optionally be setup to act as the gateway for the network. `bridge` specific configuration fields are:

- **bridge** (string): the name of the bridge to create and/or plug into. Defaults to `rkt0`.
- **isGateway** (boolean): whether the bridge should be assigned an IP and act as a gateway.
- **mtu** (integer): the size of the MTU in bytes for bridge and veths.
- **ipMasq** (boolean): whether to setup IP masquerading on the host.

#### macvlan

macvlan "clones" a real interface by assigning a made up MAC address to the "cloned" interface.
The real and macvlan interfaces share the same physical device but have distinct MAC and IP addresses.
With multiple macvlan interfaces sharing the same device, it behaves similarly to a bridge.
Since macvlan interface has its own MAC and is located on the same link segment as the host, it makes it especially a good choice for using the DHCP server to acquire an IP address.
With the IP address allocated by the real network infrastructure, this makes the pod IP routable in the same way as the host IP. `macvlan` specific configuration fields are:

- **master** (string): the name of host interface to "clone". This field is required.
- **mode** (string): One of "bridge", "private", "vepa", or "passthru". This controls how traffic is handled between different macvlan interfaces on the same host. See [this guide](http://www.pocketnix.org/posts/Linux%20Networking:%20MAC%20VLANs%20and%20Virtual%20Ethernets) for discussion of modes. Defaults to "bridge".
- **mtu** (integer): the size of the MTU in bytes for bridge and veths. Defaults to MTU of the master device.
- **ipMasq** (boolean): whether to setup IP masquerading on the host. Defaults to false.

#### ipvlan

- [Coming soon](https://github.com/coreos/rkt/issues/479)

## IP Address Management

The policy for IP address allocation, associated gateway and routes is separately configurable via the `ipam` section of the configuration file.
rkt currently ships with one type of IPAM (host-local) but DHCP is in the works. Like the network types, IPAM types can be implemented by third-parties via plugins.

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

### dhcp

[Coming soon](https://github.com/coreos/rkt/issues/558)

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
$ rkt run --private-net --port=http:8888 myapp.aci
````

Now, any traffic arriving on host's TCP port 8888 will be forwarded to the pod on port 80.

### Overriding default network
If a network has a name "default", it will override the default network added
by rkt. It is strongly recommended that such network also has type "veth" as
it protects from the pod spoofing its IP address and defeating identity
management provided by the metadata service.
