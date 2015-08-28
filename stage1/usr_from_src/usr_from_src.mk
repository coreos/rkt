UFS_SYSTEMD_DESC := $(RKT_STAGE1_SYSTEMD_SRC)/$(RKT_STAGE1_SYSTEMD_VER)
UFS_SYSTEMDDIR := $(BUILDDIR)/tmp/usr_from_src/systemd/$(call escape-for-file,$(UFS_SYSTEMD_DESC))
UFS_SYSTEMD_SRCDIR := $(UFS_SYSTEMDDIR)/src
UFS_SYSTEMD_BUILDDIR := $(UFS_SYSTEMDDIR)/build

$(call setup-dep-file,UFS_PATCHES_DEPMK,$(UFS_SYSTEMD_DESC)-systemd-patches)
$(call setup-dep-file,UFS_ROOTFS_DEPMK,$(UFS_SYSTEMD_DESC)-systemd-install)

UFS_ROOTFSDIR := $(UFS_SYSTEMDDIR)/rootfs

# We assume that the name passed to --stage1-systemd-version that
# matches a regexp '^v\d+$' (name starts with a v followed by a
# number, like v211) is a name of tag. Otherwise it's a branch. `expr
# string : regexp` returns a number of characters that matched, so if
# that number is equal to string length then it means that string
# matched the regexp.
UFS_SYSTEMD_TAG_MATCH := $(shell expr "$(RKT_STAGE1_SYSTEMD_VER)" : 'v[[:digit:]]\+')
UFS_SYSTEMD_TAG_LENGTH := $(shell expr length "$(RKT_STAGE1_SYSTEMD_VER)")
UFS_PATCHES_DIR := $(MK_SRCDIR)/patches/$(RKT_STAGE1_SYSTEMD_VER)
UFS_LIB_SYMLINK := $(ACIROOTFSDIR)/lib
UFS_LIB64_SYMLINK := $(ACIROOTFSDIR)/lib64

$(call setup-stamp-file,UFS_STAMP)
$(call setup-stamp-file,UFS_PATCHES_DEPS_STAMP,$(UFS_SYSTEMD_DESC)-systemd-patches-deps)
$(call setup-stamp-file,UFS_ROOTFS_DEPS_STAMP,$(UFS_SYSTEMD_DESC)-systemd-install-deps)
$(call setup-stamp-file,UFS_ROOTFS_STAMP,/rootfs)
$(call setup-stamp-file,UFS_SYSTEMD_CLONE_AND_PATCH_STAMP,/systemd_clone_and_patch/$(UFS_SYSTEMD_DESC))
$(call setup-stamp-file,UFS_SYSTEMD_BUILD_STAMP,/systemd_build/$(UFS_SYSTEMD_DESC))

STAGE1_USR_STAMPS += $(UFS_STAMP)
# INSTALL_SYMLINKS += usr/lib:$(UFS_LIB_SYMLINK) usr/lib64:$(UFS_LIB64_SYMLINK)
STAGE1_COPY_SO_DEPS := yes

$(call inc-one,bash.mk)

$(UFS_STAMP): $(UFS_ROOTFS_STAMP) $(UFS_ROOTFS_DEPS_STAMP) $(UFS_PATCHES_DEPS_STAMP)
	touch "$@"

$(call forward-vars,$(UFS_ROOTFS_STAMP), \
	UFS_ROOTFSDIR ACIROOTFSDIR RKT_STAGE1_SYSTEMD_VER)
# $(UFS_ROOTFS_STAMP): | $(UFS_LIB_SYMLINK) $(UFS_LIB64_SYMLINK)
$(UFS_ROOTFS_STAMP): $(UFS_SYSTEMD_BUILD_STAMP) | $(ACIROOTFSDIR)
	set -e; \
	cp -af "$(UFS_ROOTFSDIR)/." "$(ACIROOTFSDIR)"; \
	ln -sf 'src' "$(ACIROOTFSDIR)/flavor"; \
	echo "$(RKT_STAGE1_SYSTEMD_VER)" >"$(ACIROOTFSDIR)/systemd-version"; \
	touch "$@"

# We can generate initial rootfs deps after the rootfs is actually
# populated.
$(UFS_ROOTFS_DEPS_STAMP): $(UFS_ROOTFS_STAMP)
$(call generate-glob-deps,$(UFS_ROOTFS_DEPS_STAMP),$(UFS_ROOTFS_STAMP),$(UFS_ROOTFS_DEPMK),,$$$$(find "$(UFS_ROOTFSDIR)" -type f))

$(call forward-vars,$(UFS_SYSTEMD_BUILD_STAMP), \
	UFS_SYSTEMD_BUILDDIR UFS_SYSTEMD_SRCDIR MAKE UFS_ROOTFSDIR)
$(UFS_SYSTEMD_BUILD_STAMP): $(UFS_SYSTEMD_CLONE_AND_PATCH_STAMP)
	set -e; \
	rm -Rf "$(UFS_SYSTEMD_BUILDDIR)"; \
	mkdir -p "$(UFS_SYSTEMD_BUILDDIR)"; \
	pushd "$(UFS_SYSTEMD_BUILDDIR)"; \
	"$(abspath $(UFS_SYSTEMD_SRCDIR))/configure" \
		--disable-dbus \
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
		--enable-seccomp; \
	$(MAKE) all; \
	DESTDIR="$(abspath $(UFS_ROOTFSDIR))" $(MAKE) install-strip; \
	popd; \
	touch "$@"

$(UFS_SYSTEMD_CLONE_AND_PATCH_STAMP): $(UFS_SYSTEMD_SRCDIR)/configure
	touch "$@"

$(call forward-vars,$(UFS_SYSTEMD_SRCDIR)/configure, \
	UFS_PATCHES_DIR GIT UFS_SYSTEMD_SRCDIR)
$(UFS_SYSTEMD_SRCDIR)/configure:
	@set -e; \
	shopt -s nullglob ; \
	if [ -d "$(UFS_PATCHES_DIR)" ]; then \
		for p in "$(abspath $(UFS_PATCHES_DIR))"/*.patch; do \
			"$(GIT)" -C "$(UFS_SYSTEMD_SRCDIR)" am "$${p}"; \
		done; \
	fi; \
	pushd "$(UFS_SYSTEMD_SRCDIR)"; \
	./autogen.sh; \
	popd

$(call generate-glob-deps,$(UFS_PATCHES_DEPS_STAMP),$(UFS_SYSTEMD_SRCDIR)/configure,$(UFS_PATCHES_DEPMK),.patch,"$(UFS_PATCHES_DIR)"/*.patch)

$(call forward-vars,$(UFS_SYSTEMD_SRCDIR)/configure.ac, \
	GIT RKT_STAGE1_SYSTEMD_VER RKT_STAGE1_SYSTEMD_SRC UFS_SYSTEMD_SRCDIR)
$(UFS_SYSTEMD_SRCDIR)/configure.ac:
	"$(GIT)" clone --depth 1 --branch "$(RKT_STAGE1_SYSTEMD_VER)" "$(RKT_STAGE1_SYSTEMD_SRC)" "$(UFS_SYSTEMD_SRCDIR)"

ifneq ($(UFS_SYSTEMD_TAG_MATCH),$(UFS_SYSTEMD_TAG_LENGTH))

# If the name is not a tag then we try to pull new changes from upstream.

GR_TARGET := $(UFS_SYSTEMD_SRCDIR)/configure
GR_SRCDIR := $(UFS_SYSTEMD_SRCDIR)
GR_BRANCH := $(RKT_STAGE1_SYSTEMD_VER)
GR_PREREQS := $(UFS_SYSTEMD_SRCDIR)/configure.ac

include makelib/git-refresh.mk

else

# The name is a tag, so we do not refresh the git repository.

$(UFS_SYSTEMD_SRCDIR)/configure: $(UFS_SYSTEMD_SRCDIR)/configure.ac

endif

$(call undefine-namespaces,UFS)
