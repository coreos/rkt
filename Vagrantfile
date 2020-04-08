boxes=[
  {
    :hostname => "rkt",
    :box => "ubuntu/xenial64",
    :memory => 1024,
    :provision_script => "scripts/install-rkt.sh",

  },
  {
    :hostname => "dev",
    :box => "ubuntu/xenial64",
    # An e2e rkt build requires 4GB memory
    :memory => 4096,
    :provision_script => "scripts/install-deps-debian-sid.sh",
  }
]

Vagrant.configure('2') do |config|
  config.vm.synced_folder ".", "/home/ubuntu/golang/src/github.com/coreos/rkt", type: "rsync"

  boxes.each do |machine|
    config.vm.define machine[:hostname] do |node|
      node.vm.box = machine[:box]
      node.vm.hostname = machine[:hostname]
      node.vm.network "private_network", type: "dhcp"

      node.vm.provider :virtualbox do |vb, override|
        vb.memory = machine[:memory]

        # fix issues with slow dns http://serverfault.com/a/595010
        vb.customize ["modifyvm", :id, "--natdnshostresolver1", "on"]
        vb.customize ["modifyvm", :id, "--natdnsproxy1", "on"]
      end

      node.vm.provider :libvirt do |lv, override|
        lv.memory = machine[:memory]
      end

      node.vm.provider :vmware_fusion do |vf, override|
        vf.memory = machine[:memory]
      end

      if machine[:provision_script] != nil
        node.vm.provision :shell, :privileged => true, :path => machine[:provision_script]
      end
    end
  end
end
