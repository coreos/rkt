# rkt functional tests

This directory contains a set of functional tests for rkt.
The tests are oriented around [expect](http://en.wikipedia.org/wiki/Expect) scripts which spawn various `rkt run` commands and look for expected output.

## Semaphore

The tests run on the [Semaphore](https://semaphoreci.com/) CI system through the [`rktbot`](https://semaphoreci.com/rktbot) user, which is part of the [`coreos`](https://semaphoreci.com/coreos/) org on Semaphore. 
This user is authorized against the corresponding [`rktbot`](https://github.com/rktbot) GitHub account.
The credentials for `rktbot` are currently managed by CoreOS.

### Build settings

Use the following build commands:

```
./tests/install-deps.sh      # Setup
./build                      # Setup
./test                       # Thread #1
```

### Platform

Select `Ubuntu 14.04 LTS v1503 (beta with Docker support)`.

### Environment variables

```
RKT_STAGE1_USR_FROM=src
```

## CircleCI

Ideally the tests will also run on [CircleCI](https://circleci.com), but there is currently a known issue because mknod is restricted - more info [here](https://github.com/coreos/rkt/issues/600#issuecomment-87655911)

Assuming this can be resolved, the following configuration can be used:

```circle.yml
test:
  override:
    - ./tests/install-deps.sh
    - RKT_STAGE1_USR_FROM=src ./build
    - ./test
```
