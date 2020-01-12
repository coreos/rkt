#!/bin/bash -eu

# Helper script for Continuous Integration Services

# Check if the last commit message requests the CI to skip the build.
git log HEAD~..HEAD > last-commit
if grep -qE '\[ci skip\]|\[skip ci\]' last-commit ; then
    cat last-commit
    echo
    echo "Build skipped as requested in the last commit."
    touch ci-skip
    exit 0
fi

if [ "${CI-}" == true ] ; then
	# https://semaphoreci.com/
	if [ "${SEMAPHORE-}" == true ] ; then
		# Most dependencies are already installed on Semaphore.
		# Here we can install any missing dependencies. Whenever
		# Semaphore installs more dependencies on their
		# platform, they should be removed from here to save time.
    if [ $(cat /etc/lsb-release |grep trusty |wc -l) -eq 1 ]; then
      install-package libacl1-dev bc libsystemd-journal-dev
    else
		  install-package libacl1-dev bc libsystemd-dev
    fi
		# libmount: https://github.com/systemd/systemd/pull/986#issuecomment-138451264
		sudo add-apt-repository --yes ppa:pitti/systemd-semaphore
		sudo apt-get update -qq || true
		install-package libmount-dev libmount1
		
	#	sudo add-apt-repository --yes ppa:longsleep/golang-backports
	#	sudo apt-get update -qq || true
	#	install-package golang-go
		
		# building systemd v229 crashes with the gcc 4.8, update to gcc 5
		sudo add-apt-repository ppa:ubuntu-toolchain-r/test -y
		sudo apt-get update -qq || true
		install-package gcc-5 gcc-5-base libgcc-5-dev g++-5 libstdc++-5-dev libseccomp-dev
		sudo update-alternatives --remove-all gcc
		sudo update-alternatives --install /usr/bin/gcc gcc /usr/bin/gcc-5 20
		sudo update-alternatives --install /usr/bin/g++ g++ /usr/bin/g++-5 20
    gofmt -w ./rkt/
	fi
fi
