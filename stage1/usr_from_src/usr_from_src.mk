UFS_SYSTEMD_DESC := $(RKT_STAGE1_SYSTEMD_SRC)/$(RKT_STAGE1_SYSTEMD_VER)

$(call setup-tmp-dir,UFS_TMPDIR)

UFS_SYSTEMDDIR_REST := systemd/$(call escape-for-file,$(UFS_SYSTEMD_DESC))
UFS_SYSTEMDDIR := $(UFS_TMPDIR)/$(UFS_SYSTEMDDIR_REST)
UFS_SYSTEMD_SRCDIR := $(UFS_SYSTEMDDIR)/src
UFS_SYSTEMD_BUILDDIR := $(UFS_SYSTEMDDIR)/build

$(call setup-dep-file,UFS_PATCHES_DEPMK,$(UFS_SYSTEMD_DESC)-systemd-patches)
$(call setup-dep-file,UFS_ROOTFS_DEPMK,$(UFS_SYSTEMD_DESC)-systemd-install)

UFS_ROOTFSDIR := $(UFS_SYSTEMDDIR)/rootfs

$(call setup-filelist-file,UFS_SYSTEMD_SRCDIR_FILELIST,/src/$(UFS_SYSTEMD_DESC))
$(call setup-filelist-file,UFS_SYSTEMD_BUILDDIR_FILELIST,/build/$(UFS_SYSTEMD_DESC))
$(call setup-filelist-file,UFS_ROOTFSDIR_FILELIST,/rootfs/$(UFS_SYSTEMD_DESC))
$(call setup-filelist-file,UFS_PATCHES_FILELIST,/patches/$(UFS_SYSTEMD_DESC))

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

$(call setup-stamp-file,UFS_STAMP,/systemd/$(UFS_SYSTEMD_DESC))
$(call setup-stamp-file,UFS_PATCHES_DEPS_STAMP,$(UFS_SYSTEMD_DESC)-systemd-patches-deps)
$(call setup-stamp-file,UFS_ROOTFS_DEPS_STAMP,$(UFS_SYSTEMD_DESC)-systemd-install-deps)
$(call setup-stamp-file,UFS_ROOTFS_STAMP,/systemd-rootfs/$(UFS_SYSTEMD_DESC))
$(call setup-stamp-file,UFS_SYSTEMD_CLONE_AND_PATCH_STAMP,/systemd_clone_and_patch/$(UFS_SYSTEMD_DESC))
$(call setup-stamp-file,UFS_SYSTEMD_BUILD_STAMP,/systemd_build/$(UFS_SYSTEMD_DESC))
$(call setup-stamp-file,UFS_SYSTEMD_INSTALL_STAMP,/systemd_install/$(UFS_SYSTEMD_DESC))

$(call setup-clean-file,UFS_SYSTEMD_SRCDIR_CLEANMK,/src-$(UFS_SYSTEMD_DESC))
$(call setup-clean-file,UFS_SYSTEMD_BUILDDIR_CLEANMK,/build-$(UFS_SYSTEMD_DESC))
$(call setup-clean-file,UFS_ROOTFSDIR_CLEANMK,/install-$(UFS_SYSTEMD_DESC))

$(call setup-stamp-file,UFS_SYSTEMD_SRCDIR_CLEAN_STAMP,$(UFS_SYSTEMD_DESC)-src-clean)
$(call setup-stamp-file,UFS_SYSTEMD_BUILDDIR_CLEAN_STAMP,$(UFS_SYSTEMD_DESC)-build-clean)
$(call setup-stamp-file,UFS_SYSTEMD_ROOTFSDIR_CLEAN_STAMP,$(UFS_SYSTEMD_DESC)-rootfs-clean)

STAGE1_USR_STAMPS += $(UFS_STAMP)
# INSTALL_SYMLINKS += usr/lib:$(UFS_LIB_SYMLINK) usr/lib64:$(UFS_LIB64_SYMLINK)
STAGE1_COPY_SO_DEPS := yes
CREATE_DIRS += \
	$(call dir-chain,$(UFS_TMPDIR),$(UFS_SYSTEMDDIR_REST))
CLEAN_FILES += \
	$(ACIROOTFSDIR)/systemd-version
CLEAN_DIRS += \
	$(ACIROOTFSDIR)/proc \
	$(UFS_SYSTEMD_SRCDIR) \
	$(UFS_SYSTEMD_BUILDDIR) \
	$(UFS_ROOTFSDIR)
CLEAN_SYMLINKS += $(ACIROOTFSDIR)/flavor

$(call inc-one,bash.mk)

$(UFS_STAMP): $(UFS_ROOTFS_STAMP) $(UFS_ROOTFS_DEPS_STAMP) $(UFS_PATCHES_DEPS_STAMP) $(UFS_SYSTEMD_ROOTFSDIR_CLEAN_STAMP) $(UFS_SYSTEMD_BUILDDIR_CLEAN_STAMP) $(UFS_SYSTEMD_SRCDIR_CLEAN_STAMP)
	touch "$@"

$(call forward-vars,$(UFS_ROOTFS_STAMP), \
	UFS_ROOTFSDIR ACIROOTFSDIR RKT_STAGE1_SYSTEMD_VER)
# $(UFS_ROOTFS_STAMP): | $(UFS_LIB_SYMLINK) $(UFS_LIB64_SYMLINK)
$(UFS_ROOTFS_STAMP): $(UFS_SYSTEMD_INSTALL_STAMP) | $(ACIROOTFSDIR)
	set -e; \
	cp -af "$(UFS_ROOTFSDIR)/." "$(ACIROOTFSDIR)"; \
	ln -sf 'src' "$(ACIROOTFSDIR)/flavor"; \
	echo "$(RKT_STAGE1_SYSTEMD_VER)" >"$(ACIROOTFSDIR)/systemd-version"; \
	mkdir -p "$(ACIROOTFSDIR)/proc"; \
	touch "$@"

$(call forward-vars,$(UFS_SYSTEMD_INSTALL_STAMP), \
	UFS_SYSTEMD_BUILDDIR UFS_ROOTFSDIR MAKE)
$(UFS_SYSTEMD_INSTALL_STAMP): $(UFS_SYSTEMD_BUILD_STAMP)
	set -e; \
	DESTDIR="$(abspath $(UFS_ROOTFSDIR))" $(MAKE) -C "$(UFS_SYSTEMD_BUILDDIR)" install-strip; \
	touch "$@"

# This filelist can be generated only after the installation of
# systemd to temporary rootfs was performed
$(UFS_ROOTFSDIR_FILELIST): $(UFS_SYSTEMD_INSTALL_STAMP)
$(call generate-deep-filelist,$(UFS_ROOTFSDIR_FILELIST),$(UFS_ROOTFSDIR))

# Generate dep.mk file which will cause the initial ACI rootfs to be
# recreated if any file in temporary rootfs changes.
$(call generate-glob-deps,$(UFS_ROOTFS_DEPS_STAMP),$(UFS_ROOTFS_STAMP),$(UFS_ROOTFS_DEPMK),,$(UFS_ROOTFSDIR_FILELIST),$(UFS_ROOTFSDIR))

# Generate a clean.mk files for cleaning everything that systemd's
# "make install" put in a temporary rootfs and for the same files in
# ACI rootfs.
$(call generate-clean-mk,$(UFS_SYSTEMD_ROOTFSDIR_CLEAN_STAMP),$(UFS_ROOTFSDIR_CLEANMK),$(UFS_ROOTFSDIR_FILELIST),$(UFS_ROOTFSDIR) $(ACIROOTFSDIR))

$(call forward-vars,$(UFS_SYSTEMD_BUILD_STAMP), \
	UFS_SYSTEMD_BUILDDIR UFS_SYSTEMD_SRCDIR MAKE)
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
	popd; \
	touch "$@"

# Generate filelist of a build directory. This can be done only after
# building systemd was finished.
$(UFS_SYSTEMD_BUILDDIR_FILELIST): $(UFS_SYSTEMD_BUILD_STAMP)
$(call generate-deep-filelist,$(UFS_SYSTEMD_BUILDDIR_FILELIST),$(UFS_SYSTEMD_BUILDDIR))

# Generate clean.mk for cleaning all files created during "make all"
# in systemd.
$(call generate-clean-mk,$(UFS_SYSTEMD_BUILDDIR_CLEAN_STAMP),$(UFS_SYSTEMD_BUILDDIR_CLEANMK),$(UFS_SYSTEMD_BUILDDIR_FILELIST),$(UFS_SYSTEMD_BUILDDIR))

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

# Generate the filelist of systemd's srcdir. This can be done only
# after it was cloned, patched and configure script was generated.
$(UFS_SYSTEMD_SRCDIR_FILELIST): $(UFS_SYSTEMD_SRCDIR)/configure
$(call generate-deep-filelist,$(UFS_SYSTEMD_SRCDIR_FILELIST),$(UFS_SYSTEMD_SRCDIR))

# Generate clean.mk file for cleaning all files in srcdir.
$(call generate-clean-mk,$(UFS_SYSTEMD_SRCDIR_CLEAN_STAMP),$(UFS_SYSTEMD_SRCDIR_CLEANMK),$(UFS_SYSTEMD_SRCDIR_FILELIST),$(UFS_SYSTEMD_SRCDIR))

# This is a special case - normally, when generating filelists, we
# require the directory to exist. In this case, the patches directory
# may not exist and it is fine. We generate an empty filelist.
UFS_GOT_PATCHES := $(shell test -d "$(UFS_PATCHES_DIR)" && echo yes)

ifeq ($(UFS_GOT_PATCHES),yes)

# Generate shallow filelist of patches. This can happen anytime.
$(call generate-shallow-filelist,$(UFS_PATCHES_FILELIST),$(UFS_PATCHES_DIR),.patch)

else

# Generate empty filelist of patches. This can happen anytime.
$(call generate-empty-filelist,$(UFS_PATCHES_FILELIST))

endif

# Generate a dep.mk on those patches, so if patches change, the
# project should be reset and repatched, and configure script
# regenerated.
# TODO: It does not work as comment says. When patches are changed we
# try to apply them again, but instead we should do a hard reset to
# original branch and then reapply the patches.
$(call generate-glob-deps,$(UFS_PATCHES_DEPS_STAMP),$(UFS_SYSTEMD_SRCDIR)/configure,$(UFS_PATCHES_DEPMK),.patch,$(UFS_PATCHES_FILELIST),$(UFS_PATCHES_DIR),normal)

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
