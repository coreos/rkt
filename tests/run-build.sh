#!/bin/bash

set -e

export RKT_STAGE1_USR_FROM=$1
export RKT_STAGE1_SYSTEMD_VER=$2

export BUILD_DIR=build-rkt-$RKT_STAGE1_USR_FROM-$RKT_STAGE1_SYSTEMD_VER

mkdir -p builds
cd builds

# Semaphore does not clean git subtrees between each build.
sudo rm -rf $BUILD_DIR

git clone --depth 1 ../ $BUILD_DIR

cd $BUILD_DIR

./autogen.sh
./configure --with-stage1=$RKT_STAGE1_USR_FROM --with-stage1-systemd-version=$RKT_STAGE1_SYSTEMD_VER
make test

cd ..

# Make sure there is enough disk space for the next build
sudo rm -rf $BUILD_DIR

