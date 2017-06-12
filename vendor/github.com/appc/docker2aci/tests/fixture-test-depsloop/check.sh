#!/bin/sh

DOCKER2ACI=../bin/docker2aci
TESTDIR=$1
TESTNAME=$2

timeout 10s ${DOCKER2ACI} "${TESTDIR}/${TESTNAME}/${TESTNAME}.docker"
if [ $? -eq 1 ]; then
	echo "### Test case ${TESTNAME}: SUCCESS"
	exit 0
else
	echo "### Test case ${TESTNAME}: FAIL"
	exit 1
fi

