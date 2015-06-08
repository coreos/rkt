# build systemd from source, produce an install hook selecting the needed files and host dependencies

# make needs to use bash for nullglob
SHELL := /bin/bash

usr.done: Makefile aci-manifest install
	cp -f install ../aggregate/install.d/00usr
	touch usr.done

# prepare the install file
install: Makefile host_deps.txt
	echo mkdir -p "\$${ROOT}" > install.tmp
	echo cp -af ../usr_from_src/systemd_build/installed/. \"\$${ROOT}\" >> install.tmp
	cat host_deps.txt | while read dep; do echo install -D \"$${dep}\" \"\$${ROOT}/$${dep}\"; done >> install.tmp
	echo "echo $(RKT_STAGE1_SYSTEMD_VER) > \"\$${ROOT}/systemd-version\"" >> install.tmp
	mv install.tmp install

aci-manifest: aci-manifest.in
	sed 's|#SYSTEMD_VERSION#|$(RKT_STAGE1_SYSTEMD_VER)|' aci-manifest.in > aci-manifest

# discover host library dependencies for all the ELF executables in systemd_build/installed, note the LD_LIBRARY_PATH= to find systemd-produced libraries.
host_deps.txt: Makefile systemd.done bash.done
	cd systemd_build/installed && find -type f | xargs file  | grep ELF | cut -f1 -d: | LD_LIBRARY_PATH=usr/lib xargs ldd | grep -v ^\\. | grep '/' | sed  -e 's/^[[:space:]]*//' -e 's/.*=> //' -e 's/ (0x[0-9a-f]*)$$//' | grep -v ^[^/] | sort -u > ../../host_deps.txt.tmp
	mv host_deps.txt.tmp host_deps.txt

# grab bash from the host
bash.done: Makefile systemd.done
	cp -af `which bash` systemd_build/installed/usr/bin/bash
	touch bash.done

# configure, build, and install systemd
systemd.done: Makefile systemd.src
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
		&& $(MAKE) && DESTDIR=$${PWD}/installed make install-strip
	touch systemd.done

# TODO(vc): it may make more sense to have the systemd source be a git submodule?
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
			if patch -p1 <"$$p" ; \
			then \
				: ; \
			else \
				exit 1 ; \
			fi ; \
		done ; \
	fi
	cd systemd && ./autogen.sh
	touch systemd.src

.PHONY: clean distclean
clean:
	rm -Rf systemd_build systemd.done bash.done host_deps.txt rootfs usr.done install aci-manifest.in

distclean: clean
	rm -Rf systemd systemd.src

test:
	echo TODO
