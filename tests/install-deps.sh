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
		# A colon to guard against an empty body error.
		:

		# Most dependencies are already installed on Semaphore.
		# Here we can install any missing dependencies. Whenever
		# Semaphore installs more dependencies on their platform,
		# they should be removed from here to save time.

		# If there is some dependency to install then
		# uncomment the following line and add "sudo apt-get
		# install -y <dep>" after it.

		#sudo apt-get update -qq || true

		# Fix for go 1.4.3: see https://github.com/coreos/rkt/issues
		# GOPATH is only needed to allow go to work; the actual
		# binaries are installed in GOROOT for these commands
		sudo -E GOPATH=/tmp go get golang.org/x/tools/cmd/vet
		sudo -E GOPATH=/tmp go get golang.org/x/tools/cmd/cover

		# libmount: https://github.com/systemd/systemd/pull/986#issuecomment-138451264
		# sudo add-apt-repository --yes ppa:pitti/systemd-semaphore
		# sudo apt-get update -qq || true
		# sudo apt-get install -y libmount-dev libmount1
	fi
fi
