#!/bin/bash

set -e

RKT_STAGE1_USR_FROM=$1
RKT_STAGE1_SYSTEMD_VER=$2

BUILD_DIR=build-rkt-$RKT_STAGE1_USR_FROM-$RKT_STAGE1_SYSTEMD_VER

mkdir -p builds
cd builds

# Semaphore does not clean git subtrees between each build.
sudo rm -rf $BUILD_DIR

git clone --depth 1 ../ $BUILD_DIR

cd $BUILD_DIR

./autogen.sh
if [ "$RKT_STAGE1_USR_FROM" = 'src' ]
then
    ./configure --with-stage1=$RKT_STAGE1_USR_FROM --with-stage1-systemd-version=$RKT_STAGE1_SYSTEMD_VER --enable-functional-tests
else
    ./configure --with-stage1=$RKT_STAGE1_USR_FROM --enable-functional-tests
fi
CORES=$(grep -c ^processor /proc/cpuinfo)
make -j${CORES}
make check

cd ..

# Make sure there is enough disk space for the next build
sudo rm -rf $BUILD_DIR
