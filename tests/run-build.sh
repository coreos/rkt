#!/bin/bash

set -e

RKT_STAGE1_USR_FROM=$1
RKT_STAGE1_SYSTEMD_VER=$2

ORIGIN=$PWD
BUILD_DIR=builds/build-rkt-$RKT_STAGE1_USR_FROM-$RKT_STAGE1_SYSTEMD_VER/src/github.com/coreos/

mkdir -p $BUILD_DIR
cd $BUILD_DIR

# Semaphore does not clean git subtrees between each build.
sudo rm -rf rkt

git clone $ORIGIN rkt

cd rkt

./autogen.sh
if [ "$RKT_STAGE1_USR_FROM" = 'none' ] ; then
    ./configure --with-stage1=$RKT_STAGE1_USR_FROM
elif [ "$RKT_STAGE1_USR_FROM" = 'src' ] ; then
    ./configure --with-stage1=$RKT_STAGE1_USR_FROM --with-stage1-systemd-version=$RKT_STAGE1_SYSTEMD_VER --enable-functional-tests
else
    ./configure --with-stage1=$RKT_STAGE1_USR_FROM --enable-functional-tests
fi
CORES=$(grep -c ^processor /proc/cpuinfo)
make -j${CORES}
make check

cd $ORIGIN

# Make sure there is enough disk space for the next build
sudo rm -rf $BUILD_DIR
