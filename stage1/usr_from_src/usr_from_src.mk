# build systemd from source, produce an install hook selecting the needed files and host dependencies
include ../../makelib/lib.mk

PWD := $(shell pwd)
ISCRIPT := $(BUILDDIR)/install.d/00usr.install

.PHONY: install

install: SHELL := $(BASH)
install: $(BUILDDIR)/systemd.done 
	@echo $(call dep-copy-fs,$(BUILDDIR)/systemd-installed) > $(ISCRIPT)
	@echo $(call dep-install-file,$(call find-so-deps, $(BUILDDIR)/systemd-installed)) >> $(ISCRIPT)
	@echo $(call dep-systemd-version,$(RKT_STAGE1_SYSTEMD_VER)) >> $(ISCRIPT)
	@echo $(call dep-flavor,src) >> $(ISCRIPT)


ifeq ($(RKT_STAGE1_DBUS),no)
RKT_STAGE1_SYSTEMD_DBUS_FLAGS := --disable-dbus
else
RKT_STAGE1_SYSTEMD_DBUS_FLAGS := --enable-dbus
endif

# configure, build, and install systemd
$(BUILDDIR)/systemd.done: $(BUILDDIR)/systemd.src.done usr_from_src.mk
	{ [ ! -e $(BUILDDIR)/systemd-build ] || rm -Rf $(BUILDDIR)/systemd-build; }
	mkdir $(BUILDDIR)/systemd-build
	cd $(BUILDDIR)/systemd-build && $(BUILDDIR)/src/systemd/configure \
		$(RKT_STAGE1_SYSTEMD_DBUS_FLAGS) \
		--disable-python-devel \
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
	    --enable-seccomp \
		&& $(MAKE) && DESTDIR=$(BUILDDIR)/systemd-installed make install-strip
	touch $(BUILDDIR)/systemd.done

# make needs to use bash for nullglob
$(BUILDDIR)/systemd.src.done: SHELL := $(shell which $(BASH))

# TODO(vc): it may make more sense to have the systemd source be a git submodule?
$(BUILDDIR)/systemd.src.done: patches/*
	if [ -z "$(RKT_STAGE1_SYSTEMD_SRC)" ]; then echo "Error: RKT_STAGE1_SYSTEMD_SRC undefined"; exit 1; fi
	if [ -z "$(RKT_STAGE1_SYSTEMD_VER)" ]; then echo "Error: RKT_STAGE1_SYSTEMD_VER undefined"; exit 1; fi
	{ [ ! -e $(BUILDDIR)/src/systemd ] || rm -Rf $(BUILDDIR)/src/systemd; }
	if [ "$(RKT_STAGE1_SYSTEMD_VER)" = "HEAD" ]; then \
		$(GIT) clone --depth 1 $(RKT_STAGE1_SYSTEMD_SRC) $(BUILDDIR)/src/systemd; \
		PATCHES_DIR=$(PWD)/patches/master ; \
	else \
		$(GIT) clone --branch $(RKT_STAGE1_SYSTEMD_VER) --depth 1 $(RKT_STAGE1_SYSTEMD_SRC) $(BUILDDIR)/src/systemd ; \
		PATCHES_DIR=$(PWD)/patches/$(RKT_STAGE1_SYSTEMD_VER) ; \
	fi ; \
	if [ -d $$PATCHES_DIR ]; then \
		set -e ; \
		cd $(BUILDDIR)/src/systemd ; \
		shopt -s nullglob ; \
		for p in $$PATCHES_DIR/*.patch ; \
		do \
			if patch -p1 <"$$p" ; \
			then \
				: ; \
			else \
				exit 1 ; \
			fi ; \
		done ; \
	fi
	cd $(BUILDDIR)/src/systemd && ./autogen.sh
	touch $(BUILDDIR)/systemd.src.done
