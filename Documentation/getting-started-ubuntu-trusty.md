# Getting Started with Rocket on Ubuntu Trusty

The following guide will show you how to build and run the sample [etcd aci](https://github.com/coreos/etcd/releases/download/v0.5.0-alpha.4/etcd-v0.5.0-alpha.4-linux-amd64.aci) on the standard vagrantcloud.com [box for ubuntu trusty](https://vagrantcloud.com/ubuntu/boxes/trusty64).


## Download and start an Ubuntu Trusty box

```
vagrant init ubuntu/trusty64
vagrant up --provider virtualbox
```

## SSH into the VM, Install Rocket and Download the ACI

```
vagrant ssh
sudo su

curl -L https://github.com/coreos/rocket/releases/download/v0.1.1/rocket-v0.1.1.tar.gz -o rocket-v0.1.1.tar.gz
tar xzvf rocket-v0.1.1.tar.gz
cd rocket-v0.1.1
./rkt fetch https://github.com/coreos/etcd/releases/download/v0.5.0-alpha.4/etcd-v0.5.0-alpha.4-linux-amd64.aci

```

## Run the ACI

Now try running the container and you should see output like this:

```
./rkt run https://github.com/coreos/etcd/releases/download/v0.5.0-alpha.4/etcd-v0.5.0-alpha.4-linux-amd64.aci
/etc/localtime is not a symlink, not updating container timezone.
2014/12/02 08:13:11 no data-dir provided, using default data-dir ./default.etcd
2014/12/02 08:13:11 etcd: listening for peers on http://localhost:2380
2014/12/02 08:13:11 etcd: listening for peers on http://localhost:7001
2014/12/02 08:13:11 etcd: listening for client requests on http://localhost:2379
2014/12/02 08:13:11 etcd: listening for client requests on http://localhost:4001
2014/12/02 08:13:11 etcdserver: name = default
2014/12/02 08:13:11 etcdserver: data dir = default.etcd
2014/12/02 08:13:11 etcdserver: snapshot count = 10000
2014/12/02 08:13:11 etcdserver: advertise client URLs = http://localhost:2379,http://localhost:4001
2014/12/02 08:13:11 etcdserver: initial advertise peer URLs = http://localhost:2380,http://localhost:7001
2014/12/02 08:13:11 etcdserver: initial cluster = default=http://localhost:2380,default=http://localhost:7001
2014/12/02 08:13:11 etcdserver: start member ce2a822cea30bfca in cluster 7e27652122e8b2ae
2014/12/02 08:13:11 etcdserver: added local member ce2a822cea30bfca [http://localhost:2380 http://localhost:7001] to cluster 7e27652122e8b2ae
2014/12/02 08:13:12 raft: elected leader ce2a822cea30bfca at term 1
2014/12/02 08:13:12 etcdserver: published {Name:default ClientURLs:[http://localhost:2379 http://localhost:4001]} to cluster 7e27652122e8b2ae
```

You are now running etcd inside of a rocket container.
