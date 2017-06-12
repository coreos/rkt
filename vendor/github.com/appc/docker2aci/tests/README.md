# docker2aci tests

## Semaphore

The tests run on the [Semaphore](https://semaphoreci.com/) CI system.

The tests are executed on Semaphore at each Pull Request (PR).
Each GitHub PR page should have a link to the [test results on Semaphore](https://semaphoreci.com/appc/docker2aci).

### Build settings

The tests will run on two VMs.
The "Setup" and "Post thread" sections will be executed on both VMs.
The "Thread 1" and "Thread 2" will be executed in parallel in separate VMs.

#### Setup

```
./build.sh
```

#### Thread 1

```
./tests/test.sh
```

### Platform

Select `Ubuntu 14.04 LTS v1503 (beta with Docker support)`.
The platform with *Docker support* means the tests will run in a VM.

