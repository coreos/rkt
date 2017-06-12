#!/usr/bin/env bash
set -e

# Gets the directory that this script is stored in.
# https://stackoverflow.com/questions/59895/can-a-bash-script-tell-what-directory-its-stored-in
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

ORG_PATH="github.com/appc"
REPO_PATH="${ORG_PATH}/docker2aci"
VERSION=$(git describe --dirty --always)
GLDFLAGS="-X ${REPO_PATH}/lib.Version=${VERSION}"

if [ ! -h ${DIR}/gopath/src/${REPO_PATH} ]; then
  mkdir -p ${DIR}/gopath/src/${ORG_PATH}
  cd ${DIR} && ln -s ../../../.. gopath/src/${REPO_PATH} || exit 255
fi

export GO15VENDOREXPERIMENT=1
export GOBIN=${DIR}/bin
export GOPATH=${DIR}/gopath
export GOOS GOARCH

eval $(go env)

if [ "${GOOS}" = "freebsd" ]; then
    # /usr/bin/cc is clang on freebsd, but we need to tell it to go to
    # make it generate proper flavour of code that doesn't emit
    # warnings.
    export CC=clang
fi

echo "Building docker2aci..."
go build -o ${GOBIN}/docker2aci -ldflags "${GLDFLAGS}" ${REPO_PATH}
