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
host_deps.txt: Makefile systemd.done bash.done
	set -e; \
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

# grab bash from the host
bash.done: Makefile dirs.done
	cp -af `which bash` "$(instdir)/usr/bin/bash"
	touch bash.done

# configure, build, and install systemd
systemd.done: Makefile systemd.src dirs.done
	{ [ ! -e systemd_build ] || rm -Rf systemd_build; }
	mkdir systemd_build
	cd systemd_build && ../systemd/configure \
		--disable-python-devel \
		--disable-dbus \
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
		dirs.done

distclean: clean
	rm -Rf systemd systemd.src

test:
	echo TODO
