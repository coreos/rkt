#!/bin/bash -eu

# Helper script for Continuous Integration Services

if [ "${CI-}" == true ] ; then
	# https://semaphoreci.com/
	if [ "${SEMAPHORE-}" == true ] ; then
		# Most dependencies are already installed on Semaphore.
		# Here we can install any missing dependencies. Whenever
		# Semaphore installs more dependencies on their platform,
		# they should be removed from here to save time.

		sudo apt-get update -qq || true
		sudo apt-get install -y libseccomp-dev libseccomp2 # systemd deps
		sudo apt-get install -y libcapture-tiny-perl # used by tools/
	fi

	# https://circleci.com/
	if [ "${CIRCLECI-}" == true ] ; then
		sudo apt-get update -qq
		sudo apt-get install -y libtool intltool # for ./autogen.sh
		sudo apt-get install -y cpio realpath squashfs-tools
		sudo apt-get install -y strace gdb libcap-ng-utils
		sudo apt-get install -y coreutils # systemd needs a recent /bin/ln
		sudo apt-get install -y gperf libcap-dev linux-headers-3.13.0-34-generic linux-libc-dev libseccomp-dev # systemd deps
		sudo apt-get install -y git # cloning a tag of systemd
	fi
fi
