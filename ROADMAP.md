# rkt roadmap

This document defines a high level roadmap for rkt development.
The dates below should not be considered authoritative, but rather indicative of the projected timeline of the project.
The [milestones defined in GitHub](https://github.com/coreos/rkt/milestones) represent the most up-to-date state of affairs.

rkt is an implementation of the [App Container spec](https://github.com/appc/spec), which is still under active development on an approximately similar timeframe.
The version of the spec that rkt implements can be seen in the output of `rkt version`.

rkt's version 1.0 release marks the command line user interface and on-disk data structures as stable and reliable for external development. The (optional) API for pod inspection is not yet completely stabilized, but is quite usable.

### rkt 1.15.0 (September)

Full plan at https://github.com/coreos/rkt/milestone/49.

Highlights:
- initial CRI support, replacing the existing gRPC [API](https://github.com/coreos/rkt/tree/master/api/v1alpha).
- Refactor distribution/storage handling
- Improved native support for OCI
- Enhanced DNS configuration
- Further improvements for SELinux environments, especially Fedora in enforcing mode
- Support for unified cgroups

### Upcoming

Full plan at https://github.com/coreos/rkt/milestone/30.

Highlights:
- `rkt fly` as top-level command
- User configuration for stage1
- Stable gRPC [API](https://github.com/coreos/rkt/tree/master/api/v1alpha)
- Packaged for more distributions
  - CentOS
