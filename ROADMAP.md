# rkt roadmap

**work in progress**

This document defines a high level roadmap for rkt development.
The dates below should not be considered authoritative, but rather indicative of the projected timeline of the project.
The [milestones defined in GitHub](https://github.com/coreos/rkt/milestones) represent the most up-to-date state of affairs.

rkt is an implementation of the [App Container spec](https://github.com/appc/spec), which is still under active development on an approximately similar timeframe.
Until the specification reaches version 1.0, rkt's major/minor versions will trail that of the spec.

### rkt 0.6 (April)
- Better UX around local image management with `rkt images`
 - inspect, extract, remove images
 - automatically fetch image dependencies
- Further functional testing
- Better UX around trust
- Per-app resource isolation

### rkt 0.7 (April/May)
- Smarter image discovery: cache control and etag handling 
- rkt registers pods with systemd-machined where possible

### rkt 0.8-0.9 (May)
- stage1 split out from coreos/rkt
- rkt packaged for other distributions (e.g. Fedora)
- rkt can run applications as non-root
- rkt can retrieve images via BitTorrent  (Have you given serious consideration to metalink?  Weather you settle on BT or metalink, I still think that this is actually one of the most revolutionary features of AppC/RKT, and I hope that eventually you'll use both.  Okay, that's my $.02.  Thanks for your hard work!)

### rkt 1.0 (June)
- Pod-level isolation
- IPv6 support
- Proper CLI/subcommand framework
