#!/usr/bin/env bash
set -e

if ! [[ "$0" =~ "tests/rkt-monitor/build-stresser.sh" ]]; then
	echo "must be run from repository root"
	exit 255
fi

stressers=(cpu mem log)

if [ -z "${1}" ]; then
    echo Specify one of \""${stressers[@]}"\"
    exit 1
fi


echo "Building worker..."
make rkt-monitor

acbuildEnd() {
    export EXIT=$?
    acbuild --debug end && exit $EXIT
}

acbuild --debug begin

trap acbuildEnd EXIT

acbuild --debug set-name appc.io/rkt-"${1}"-stresser

acbuild --debug copy build-rkt-1.13.0+git/target/bin/"${1}"-stresser /worker

acbuild --debug set-exec -- /worker

acbuild --debug write --overwrite "${1}"-stresser.aci
