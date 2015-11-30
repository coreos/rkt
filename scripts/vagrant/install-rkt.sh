#!/bin/bash

set -xe
export DEBIAN_FRONTEND=noninteractive

APP_SPEC_VERSION=0.6.1

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

which unsquashfs || sudo apt-get install -y squashfs-tools autoconf

pushd /vagrant
./autogen.sh && ./configure && make BUILDDIR=build-rkt
sudo cp -v build-rkt/bin/* /usr/local/bin
