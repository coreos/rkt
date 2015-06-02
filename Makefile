all:
	RKT_STAGE1_USR_FROM=src ./build

build:
	./build

install:
	mkdir -p $(DESTDIR)/usr/bin
	mkdir -p $(DESTDIR)/usr/share/rkt
	mkdir -p $(DESTDIR)/usr/lib/systemd/system
	cp bin/rkt $(DESTDIR)/usr/bin/rkt
	cp bin/actool $(DESTDIR)/usr/bin/actool
	cp bin/stage1.aci $(DESTDIR)/usr/bin/stage1.aci

	cp bin/bridge bin/gc bin/init bin/macvlan bin/host-local bin/veth $(DESTDIR)/usr/share/rkt

	# install metadata unitfiles
	cp dist/init/systemd/rkt-metadata.service $(DESTDIR)/usr/lib/systemd/system/rkt-metadata.service
	cp dist/init/systemd/rkt-metadata.socket $(DESTDIR)/usr/lib/systemd/system/rkt-metadata.socket

check:
	./test
