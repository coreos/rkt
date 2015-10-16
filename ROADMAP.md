# rkt roadmap

This document defines a high level roadmap for rkt development.
The dates below should not be considered authoritative, but rather indicative of the projected timeline of the project.
The [milestones defined in GitHub](https://github.com/coreos/rkt/milestones) represent the most up-to-date state of affairs.

rkt is an implementation of the [App Container spec](https://github.com/appc/spec), which is still under active development on an approximately similar timeframe. 
The version of the spec that rkt implements can be seen in the output of `rkt version`.

**Note that until communicated otherwise, the rkt CLI and API are considered unstable and subject to change.**
Backwards-compatibility is a priority for the v1.0.0 release.

### rkt v0.10 (October)
- different shared namespace execution modes [#1433](https://github.com/coreos/rkt/issues/1433)
- CLI exposes fine-grained security controls [#1568](https://github.com/coreos/rkt/issues/#1568) [#912](https://github.com/coreos/rkt/issues/912)
- API service to expose read-only information about rkt pods [#1208](https://github.com/coreos/rkt/issues/1208)
- packaged for other distributions 
  - Fedora [#1304](https://github.com/coreos/rkt/issues/1304) 
  - Debian [#1307](https://github.com/coreos/rkt/issues/1307) 

### rkt 1.0 (November?)
- stable CLI interface
- stable read-only API interface
- fully integrated with `machinectl login` and `systemd-run` [#1463](https://github.com/coreos/rkt/issues/1463)
- rkt can retrieve images via BitTorrent [#405](https://github.com/coreos/rkt/issues/405)
- packaged for more distributions
  - CentOS [#1305](https://github.com/coreos/rkt/issues/1305) 
  - OpenSUSE [#1304](https://github.com/coreos/rkt/issues/1308) 
- IPv6 support
- rkt can run applications as non-root [#820](https://github.com/coreos/rkt/issues/820)
