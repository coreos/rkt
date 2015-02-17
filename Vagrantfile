Vagrant.configure("2") do |config|
  config.vm.box = "ubuntu/trusty64"

  config.vm.provision :shell, :privileged => false, :path => "scripts/install-go.sh"

end
