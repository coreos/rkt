#!/bin/bash

set -e

# Gets the parent of the directory that this script is stored in.
# https://stackoverflow.com/questions/59895/can-a-bash-script-tell-what-directory-its-stored-in
DIR="$( cd "$( dirname $( dirname "${BASH_SOURCE[0]}" ) )" && pwd )"

ORG_PATH="github.com/appc"
REPO_PATH="${ORG_PATH}/docker2aci"

if [ ! -h ${DIR}/gopath/src/${REPO_PATH} ]; then
  mkdir -p ${DIR}/gopath/src/${ORG_PATH}
  cd ${DIR} && ln -s ../../../.. gopath/src/${REPO_PATH} || exit 255
fi

export GO15VENDOREXPERIMENT=1
export GOPATH=${DIR}/gopath

go vet ./pkg/...
go vet ./lib/...
go test -v ${REPO_PATH}/lib/tests
go test -v ${REPO_PATH}/lib/internal
go test -v ${REPO_PATH}/lib/common

DOCKER2ACI=../bin/docker2aci
PREFIX=docker2aci-tests
TESTDIR=$(dirname $(realpath $0))
RKTVERSION=v1.1.0

cd $TESTDIR

# install rkt in Semaphore
if ! which rkt > /dev/null ; then
	if [ "$SEMAPHORE" != "true" ] ; then
		echo "Please install rkt"
		exit 1
	fi
	pushd $SEMAPHORE_CACHE_DIR
	if ! md5sum -c $TESTDIR/rkt-$RKTVERSION.md5sum; then
		wget https://github.com/coreos/rkt/releases/download/$RKTVERSION/rkt-$RKTVERSION.tar.gz
	fi
	md5sum -c $TESTDIR/rkt-$RKTVERSION.md5sum
	tar xf rkt-$RKTVERSION.tar.gz
	export PATH=$PATH:$PWD/rkt-$RKTVERSION/
	popd
fi
RKT=$(which rkt)

DOCKER_STORAGE_BACKEND=$(sudo docker info|grep '^Storage Driver:'|sed 's/Storage Driver: //')

for i in $(find . -maxdepth 1 -type d -name 'fixture-test*') ; do
	  TESTNAME=$(basename $i)
	  echo "### Test case ${TESTNAME}..."
	  $TESTDIR/${TESTNAME}/check.sh "${TESTDIR}" "${TESTNAME}"
done

for i in $(find . -maxdepth 1 -type d -name 'test-*') ; do
	TESTNAME=$(basename $i)
	echo "### Test case ${TESTNAME}: build..."
	sudo docker build --tag=$PREFIX/${TESTNAME} --no-cache=true ${TESTNAME}

	echo "### Test case ${TESTNAME}: test in Docker..."
	sudo docker run --rm \
	                --env=CHECK=docker-run \
	                --env=DOCKER_STORAGE_BACKEND=$DOCKER_STORAGE_BACKEND \
	                $PREFIX/${TESTNAME}

	echo "### Test case ${TESTNAME}: converting to ACI..."
	sudo docker save -o ${TESTNAME}.docker $PREFIX/${TESTNAME}
	# Docker now writes files as root, so make them readable
	sudo chmod o+rx ${TESTNAME}.docker
	$DOCKER2ACI ${TESTNAME}.docker

	echo "### Test case ${TESTNAME}: test in rkt..."
	sudo $RKT prepare --insecure-options=image \
	                  --set-env=CHECK=rkt-run \
	                  --set-env=DOCKER_STORAGE_BACKEND=$DOCKER_STORAGE_BACKEND \
	                  ./${PREFIX}-${TESTNAME}-latest.aci \
	                  > rkt-uuid-${TESTNAME}
	sudo $RKT run-prepared $(cat rkt-uuid-${TESTNAME})
	sudo $RKT status $(cat rkt-uuid-${TESTNAME}) | grep app-${TESTNAME}=0
	sudo $RKT rm $(cat rkt-uuid-${TESTNAME})

	echo "### Test case ${TESTNAME}: test with 'rkt image render'..."
	sudo $RKT image render --overwrite ${PREFIX}/${TESTNAME} ./rendered-${TESTNAME}
	pushd rendered-${TESTNAME}/rootfs
	CHECK=rkt-rendered DOCKER_STORAGE_BACKEND=$DOCKER_STORAGE_BACKEND $TESTDIR/${TESTNAME}/check.sh
	popd
	echo "### Test case ${TESTNAME}: SUCCESS"

	sudo docker rmi $PREFIX/${TESTNAME}
done
