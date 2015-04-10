#!/bin/bash -eu

# Helper script for Continuous Integration Services

if [ "${CI-}" == true ] ; then
	# https://semaphoreci.com/
	if [ "${SEMAPHORE-}" == true ] ; then
		sudo apt-get update -qq
		sudo apt-get install -y cpio realpath squashfs-tools
		sudo apt-get install -y strace gdb libcap-ng-utils
		sudo apt-get install -y expect
		sudo apt-get install -y coreutils # systemd needs a recent /bin/ln
		sudo apt-get install -y gperf libcap-dev intltool # systemd deps
		sudo apt-get install -y git # cloning a tag of systemd
	fi

	# https://circleci.com/
	if [ "${CIRCLECI-}" == true ] ; then
		sudo apt-get update -qq
		sudo apt-get install -y cpio realpath squashfs-tools
		sudo apt-get install -y strace gdb libcap-ng-utils
		sudo apt-get install -y expect
		sudo apt-get install -y coreutils # systemd needs a recent /bin/ln
		sudo apt-get install -y gperf libcap-dev linux-headers-3.13.0-34-generic linux-libc-dev # systemd deps
		sudo apt-get install -y git # cloning a tag of systemd
	fi
fi

