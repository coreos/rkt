# rkt functional tests

This directory contains a set of functional tests for rkt.
The tests use [gexpect](https://github.com/ThomasRooney/gexpect) to spawn various `rkt run` commands and look for expected output.

## Semaphore

The tests run on the [Semaphore](https://semaphoreci.com/) CI system through the [`rktbot`](https://semaphoreci.com/rktbot) user, which is part of the [`coreos`](https://semaphoreci.com/coreos/) org on Semaphore. 
This user is authorized against the corresponding [`rktbot`](https://github.com/rktbot) GitHub account.
The credentials for `rktbot` are currently managed by CoreOS.

### Build settings

Use the following build commands:

```
./tests/install-deps.sh            # Setup
./tests/run-build.sh none          # Thread 1
./tests/run-build.sh src v220      # Thread 1
./tests/run-build.sh host          # Thread 1
./tests/run-build.sh coreos        # Thread 2
./tests/run-build.sh src master    # Thread 2
git clean -ffdx                    # Post Thread
```

### Platform

Select `Ubuntu 14.04 LTS v1503 (beta with Docker support)`.

### Environment variables

```
RKT_ENABLE_FUNCTIONAL_TESTS=true
```

## CircleCI

Ideally the tests will also run on [CircleCI](https://circleci.com), but there is currently a known issue because access to the cgroup filesystems is restricted - more info [here](https://github.com/coreos/rkt/issues/600#issuecomment-87655911)

Assuming this can be resolved, the following configuration can be used:

```circle.yml
test:
  override:
    - ./tests/install-deps.sh
    - RKT_STAGE1_USR_FROM=src ./build
    - ./test
```
