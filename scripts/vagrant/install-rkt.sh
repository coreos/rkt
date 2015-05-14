#!/bin/bash

set -xe
export DEBIAN_FRONTEND=noninteractive

APP_SPEC_VERSION=0.4.1

if ! [ -d app-spec ]; then
  echo "Install actool ${APP_SPEC_VERSION}"
  mkdir app-spec
  wget -q -O appc-spec.tar.gz https://github.com/appc/spec/archive/v${APP_SPEC_VERSION}.tar.gz
  tar xzvf appc-spec.tar.gz -C app-spec --strip-components=1
  pushd app-spec
  ./build
  sudo cp -v bin/* /usr/local/bin
  popd
fi

which unsquash || sudo apt-get install -y squashfs-tools

pushd /vagrant
./build
sudo cp -v bin/* /usr/local/bin

cat << EOF | sudo tee /etc/profile.d/99rkt.sh
  export PATH=$PWD/bin:\$PATH
  export GOPATH=$PWD/gopath:\$GOPATH
EOF
