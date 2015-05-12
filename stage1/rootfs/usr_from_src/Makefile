# build systemd from source, produce an install hook selecting the
# needed files and host dependencies

# make needs to use bash for nullglob
SHELL := /bin/bash
abs_srcdir := $(shell pwd)
instdir := $(abs_srcdir)/installed

usr.done: Makefile install
	cp -f install ../aggregate/install.d/00usr
	touch usr.done

# prepare the install file
install: Makefile host_deps.txt
	echo mkdir -p "\$${ROOT}" > install.tmp
	echo cp -af \"$(instdir)/.\" \"\$${ROOT}\" >> install.tmp
	cat host_deps.txt | while read dep; do \
		echo install -D \"$${dep}\" \"\$${ROOT}/$${dep}\"; \
	done >> install.tmp
	echo ln -sf src \"\$${ROOT}/flavor\" >> install.tmp
	mv install.tmp install

hd_tmp := $(abs_srcdir)/host_deps.txt.tmp
# discover host library dependencies for all the ELF executables in
# $(instdir), note the LD_LIBRARY_PATH= to find systemd-produced
# libraries.
host_deps.txt: Makefile systemd.done bash.done dbus.done agetty.done mount.done ldconfig.done login.done
	set -e; \
	for f in /lib64/libnss_*.so; do \
		echo cp -af "$${f}" "$(instdir)$${f}"; \
		cp -af "$${f}" "$(instdir)$${f}"; \
	done; \
	cd "$(instdir)" && \
		find -type f | \
		xargs file  | \
		grep ELF | \
		cut -f1 -d: | \
		LD_LIBRARY_PATH=usr/lib xargs ldd | \
		grep -v ^\\. | \
		grep '/' | \
		sed  -e 's/^[[:space:]]*//' -e 's/.*=> //' -e 's/ (0x[0-9a-f]*)$$//' | \
		grep -v ^[^/] | sort -u > "$(hd_tmp)"
	mv "$(hd_tmp)" host_deps.txt

# build dbus from git
dbus.done: Makefile systemd.done dbus.src
	@# build dbus
	set -e; \
	{ [ ! -e dbus_build ] || rm -Rf dbus_build; }; \
	mkdir dbus_build; \
	cd dbus_build; \
	"$(abs_srcdir)/dbus/configure" \
		--prefix=/usr \
		--sysconfdir=/etc \
		--libexecdir=/usr/lib64/dbus-1 \
		--enable-verbose-mode \
		--disable-asserts \
		--enable-checks \
		--disable-xml-docs \
		--disable-doxygen-docs \
		--disable-ducktype-docs \
		--enable-abstract-sockets \
		--disable-selinux \
		--disable-apparmor \
		--disable-libaudit \
		--enable-inotify \
		--disable-console-owner-file \
		--disable-launchd \
		--enable-systemd \
		--disable-embedded-tests \
		--disable-modular-tests \
		--disable-tests \
		--disable-installed-tests \
		--enable-epoll \
		--disable-x11-autolaunch \
		--disable-Werror \
		--enable-ld-version-script \
		--disable-stats \
		--enable-user-session \
		--with-dbus-user=root \
		--with-init-scripts=none \
		--with-system-socket=/run/dbus/system_bus_socket \
		--with-system-pid-file=/run/dbus/messagebus.pid \
		&& $(MAKE) && DESTDIR="$(instdir)" $(MAKE) install-strip

	@# misc
	set -e; \
	cp -af passwd "$(instdir)/etc/passwd"; \
	touch dbus.done

# clone dbus, apply patches
dbus.src: Makefile patches/*
	set -e; \
	{ [ ! -e dbus ] || rm -Rf dbus; }; \
	dbus_tag='dbus-1.8.16'; \
	dbus_git='git://anongit.freedesktop.org/dbus/dbus'; \
	git clone --branch "$$dbus_tag" --depth 1 "$$dbus_git"; \
	cd dbus; \
	shopt -s nullglob; \
	dpd="$(abs_srcdir)/patches/dbus"; \
	if [ -d "$${dpd}" ]; then \
		for p in "$${dpd}"/*.patch; \
		do \
			git am "$$p"; \
		done ; \
	fi; \
	./autogen.sh --no-configure
	touch dbus.src

# grab bash from the host
bash.done: Makefile dirs.done
	cp -af `which bash` "$(instdir)/usr/bin/bash"
	touch bash.done

# grab agetty from the host
agetty.done: Makefile dirs.done
	cp -af `which agetty` "$(instdir)/usr/sbin/agetty"
	touch agetty.done

login.done: Makefile dirs.done
	$(CC) $(CFLAGS) -o "$(instdir)/bin/login" login.c -static -s
	touch login.done

# grab mount from the host
mount.done: Makefile dirs.done
	cp -af `which mount` "$(instdir)/usr/bin/mount"
	touch mount.done

# grab ldconfig from the host
ldconfig.done: Makefile dirs.done
	cp -af `which ldconfig` "$(instdir)/usr/sbin/ldconfig"
	touch ldconfig.done

# configure, build, and install systemd
systemd.done: Makefile systemd.src dirs.done
	{ [ ! -e systemd_build ] || rm -Rf systemd_build; }
	mkdir systemd_build
	cd systemd_build && ../systemd/configure \
		--disable-python-devel \
		--enable-dbus \
		--disable-kmod \
		--disable-blkid \
		--disable-chkconfig \
		--disable-selinux \
		--disable-pam \
		--disable-acl \
		--disable-smack \
		--disable-gcrypt \
		--disable-elfutils \
		--disable-libcryptsetup \
		--disable-qrencode \
		--disable-microhttpd \
		--disable-gnutls \
		--disable-binfmt \
		--disable-vconsole \
		--disable-bootchart \
		--disable-quotacheck \
		--disable-tmpfiles \
		--disable-sysusers \
		--disable-randomseed \
		--disable-backlight \
		--disable-rfkill \
		--disable-logind \
		--disable-machined \
		--disable-timedated \
		--disable-timesyncd \
		--disable-localed \
		--disable-coredump \
		--disable-polkit \
		--disable-resolved \
		--disable-networkd \
		--disable-efi \
		--disable-myhostname \
		--disable-gudev \
		--disable-manpages \
		--disable-tests \
		--disable-blkid \
		--disable-hibernate \
		--disable-terminal \
		--disable-hwdb \
		--disable-importd \
		--disable-firstboot \
		&& $(MAKE) && DESTDIR="$(instdir)" $(MAKE) install-strip
	touch systemd.done

# TODO(vc): it may make more sense to have the systemd source be a git
# submodule?
systemd.src: Makefile patches/*
	if [ -z "$(RKT_STAGE1_SYSTEMD_SRC)" ]; then echo "Error: RKT_STAGE1_SYSTEMD_SRC undefined"; exit 1; fi
	if [ -z "$(RKT_STAGE1_SYSTEMD_VER)" ]; then echo "Error: RKT_STAGE1_SYSTEMD_VER undefined"; exit 1; fi
	{ [ ! -e systemd ] || rm -Rf systemd; }
	mkdir systemd
	if [ "$(RKT_STAGE1_SYSTEMD_VER)" = "HEAD" ]; then \
		git clone --depth 1 $(RKT_STAGE1_SYSTEMD_SRC) ; \
		PATCHES_DIR=patches/master ; \
	else \
		git clone --branch $(RKT_STAGE1_SYSTEMD_VER) --depth 1 $(RKT_STAGE1_SYSTEMD_SRC) ; \
		PATCHES_DIR=patches/$(RKT_STAGE1_SYSTEMD_VER) ; \
	fi ; \
	if [ -d $$PATCHES_DIR ]; then \
		set -e ; \
		cd systemd ; \
		shopt -s nullglob ; \
		for p in ../$$PATCHES_DIR/*.patch ; \
		do \
			git am "$$p"; \
		done ; \
	fi
	cd systemd && ./autogen.sh
	touch systemd.src

# create directories and symlinks
dirs.done:
	@echo "Preparing installation directory ($(instdir))"
	set -e; \
	for d in \
		/root \
		/usr/bin \
		/usr/sbin \
		/usr/lib/systemd/system/sockets.target.wants \
		/usr/lib/systemd/system/default.target.wants \
		/var \
		/run; \
	do \
		mkdir -p "$(instdir)$${d}"; \
	done; \
	for s in \
		'usr/bin;/bin' \
		'usr/lib;/lib' \
		'usr/lib64;/lib64' \
		'usr/sbin;/sbin' \
		'dbus.service;/lib/systemd/system/messagebus.service' \
		'../dbus.socket;/lib/systemd/system/sockets.target.wants/dbus.socket' \
		'../dbus.service;/lib/systemd/system/default.target.wants/dbus.service' \
		'../run;/var/run'; \
	do \
		tgt=`echo $${s} | cut -d\; -f1`; \
		sl=`echo $${s} | cut -d\; -f2`; \
		ln -s "$${tgt}" "$(instdir)$${sl}"; \
	done; \
	touch dirs.done

.PHONY: clean distclean
clean:
	rm -Rf \
		"$(instdir)" \
		systemd_build \
		systemd.done \
		bash.done \
		host_deps.txt \
		usr.done \
		install \
		dbus.done \
		dirs.done \
		agetty.done \
		mount.done \
		ldconfig.done \
		login.done

distclean: clean
	rm -Rf systemd systemd.src dbus dbus.src

test:
	echo TODO
