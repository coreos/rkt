Vagrant.configure('2') do |config|
  config.vm.box = "ubuntu/trusty64" # Ubuntu 14.04
  config.vm.provision :shell, :privileged => false, :path => "scripts/install-go.sh"
  config.vm.provision :shell, :privileged => false, :path => "scripts/install-rocket.sh"

  # set auto_update to false, if you do NOT want to check the correct 
  # additions version when booting this machine
  config.vbguest.auto_update = true

  # do NOT download the iso file from a webserver
  config.vbguest.no_remote = true
end
