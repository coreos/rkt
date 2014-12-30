#!/bin/bash -e

# aggregate everything into a single rootfs tree

RD=s1rootfs

# always start over
[ -e "$RD" ] && rm -Rf "$RD"

# start with the usr/rootfs
cp -a ../usr/rootfs "$RD"

# populate the systemd units
install -d -m 0755 "$RD/usr/lib/systemd/system"
install -d -m 0755 "$RD/usr/lib/systemd/system/default.target.wants"
install -d -m 0755 "$RD/usr/lib/systemd/system/sockets.target.wants"
install -m 0644 units/default.target "$RD/usr/lib/systemd/system"
install -m 0644 units/exit-watcher.service "$RD/usr/lib/systemd/system"
install -m 0644 units/local-fs.target "$RD/usr/lib/systemd/system"
install -m 0644 units/reaper.service "$RD/usr/lib/systemd/system"
install -m 0644 units/sockets.target "$RD/usr/lib/systemd/system"
install -m 0755 scripts/reaper.sh "$RD"
install -m 0755 ../shim/shim.so "$RD"
install -m 0755 ../enter/enter "$RD"
ln -s shim.so "$RD/fakesdboot.so"

install -d "$RD/etc"
echo "rocket" > "$RD/etc/os-release"

# parent dir for the stage2 bind mounts
install -d "$RD/opt/stage2"

# dir for result code files
install -d "$RD/rkt/status"

