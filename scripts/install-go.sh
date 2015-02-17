#!/bin/bash

set -x

export DEBIAN_FRONTEND=noninteractive
VERSION=1.4.1
OS=linux
ARCH=amd64

prefix=/usr/local

# grab go
if ! [ -e $prefix/go ]; then

    wget -q https://storage.googleapis.com/golang/go$VERSION.$OS-$ARCH.tar.gz
    sudo tar -C $prefix -xzf go$VERSION.$OS-$ARCH.tar.gz
fi

# setup user environment variables
echo "export GOROOT=$prefix/go" |sudo tee /etc/profile.d/go.sh
cat << 'EOF' |sudo tee -a /etc/profile.d/go.sh

export GOPATH=$HOME/.gopath

[ -e $GOPATH ] || mkdir -p $GOPATH

export PATH=$PATH:$GOROOT/bin:$GOPATH/bin
EOF

# not essential but go get depends on it
which git || sudo apt-get install -y git
