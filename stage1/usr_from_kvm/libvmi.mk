include stage1/usr_from_kvm/kernel-version.mk
$(call setup-stamp-file,LIBVMI_STAMP)
LIBVMI_TMPDIR := $(UFK_TMPDIR)/libvmi
LIBVMI_SRCDIR := $(LIBVMI_TMPDIR)/src
LIBVMI_BINARY := $(LIBVMI_SRCDIR)/examples/process-state-monitor
LIBVMI_ACI_BINARY := $(HV_ACIROOTFSDIR)/monitor
LIBVMI_GIT := https://github.com/libvmi/libvmi
# just last published version (for reproducible builds), not for any other reason
LIBVMI_VERSION := master
KERNEL_BUILDDIR=$(abspath $(UFK_TMPDIR)/kernel/build-$(KERNEL_VERSION)/)

LIBVMI_STUFFDIR := $(MK_SRCDIR)/libvmi
LIBVMI_PATCHESDIR := $(LIBVMI_STUFFDIR)/patches
LIBVMI_PATCHES := $(abspath $(LIBVMI_PATCHESDIR)/*.patch)

$(call setup-stamp-file,LIBVMI_BUILD_STAMP,/build)
$(call setup-stamp-file,LIBVMI_PATCH_STAMP,/patch_libvmi)
$(call setup-stamp-file,LIBVMI_DEPS_STAMP,/deps)
$(call setup-dep-file,LIBVMI_PATCHES_DEPMK)
$(call setup-filelist-file,LIBVMI_PATCHES_FILELIST,/patches)

S1_RF_SECONDARY_STAMPS += $(LIBVMI_STAMP)
S1_RF_INSTALL_FILES += $(LIBVMI_BINARY):$(LIBVMI_ACI_BINARY):-
INSTALL_DIRS += \
	$(LIBVMI_SRCDIR):- \
	$(LIBVMI_TMPDIR):-

$(call generate-stamp-rule,$(LIBVMI_STAMP),$(LIBVMI_ACI_BINARY) $(LIBVMI_DEPS_STAMP))

$(LIBVMI_BINARY): $(LIBVMI_BUILD_STAMP)

LIBVMI_CONFIGURATION_OPTS := --enable-qemu --disable-xen --disable-kvm --disable-file --disable-address-cache --disable-page-cache --no-create --no-recursion --enable-static --disable-vmifs --disable-windows --with-vmlinux=$(KERNEL_BUILDDIR)/vmlinux

LIBVMI_LD_FLAGS=-static -static-libgcc -lpthread -Wl,-Bstatic

$(call generate-stamp-rule,$(LIBVMI_BUILD_STAMP),$(LIBVMI_PATCH_STAMP),, \
	$(call vb,vt,BUILD EXT,libvmi) \
	cd $(LIBVMI_SRCDIR); \
        ./autogen.sh; ./configure $(LIBVMI_CONFIGURATION_OPTS); ./config.status; sed s/-Werror//g -i */Makefile; \
        set -x; \
        $$(MAKE) $(call vl2,--silent) -C "$(LIBVMI_SRCDIR)/libvmi" V= $(call vl2,>/dev/null); \
        $$(MAKE) LDFLAGS="$(LIBVMI_LD_FLAGS)" -C "$(LIBVMI_SRCDIR)/examples" V= process-state-monitor $(call vl2,>/dev/null))


# Generate clean file for libvmi directory (this is both srcdir and
# builddir). Can happen after build finished.
$(call generate-clean-mk-simple, \
	$(LIBVMI_STAMP), \
	$(LIBVMI_SRCDIR), \
	$(LIBVMI_SRCDIR), \
	$(LIBVMI_BUILD_STAMP), \
	cleanup)

$(call generate-stamp-rule,$(LIBVMI_PATCH_STAMP),,, \
	shopt -s nullglob; \
	for p in $(LIBVMI_PATCHES); do \
		$(call vb,v2,PATCH,$$$${p#$(MK_TOPLEVEL_ABS_SRCDIR)/}) \
		patch $(call vl3,--silent) --directory="$(LIBVMI_SRCDIR)" --strip=1 --forward <"$$$${p}"; \
	done)

# Generate a filelist of patches. Can happen anytime.
$(call generate-patches-filelist,$(LIBVMI_PATCHES_FILELIST),$(LIBVMI_PATCHESDIR))

# Generate dep.mk on patches, so if they change, the project has to be
# reset to original checkout and patches reapplied.
$(call generate-glob-deps,$(LIBVMI_DEPS_STAMP),$(LIBVMI_SRCDIR)/Makefile,$(LIBVMI_PATCHES_DEPMK),.patch,$(LIBVMI_PATCHES_FILELIST),$(LIBVMI_PATCHESDIR),normal)

# parameters for makelib/git.mk
GCL_REPOSITORY := $(LIBVMI_GIT)
GCL_DIRECTORY := $(LIBVMI_SRCDIR)
GCL_COMMITTISH := $(LIBVMI_VERSION)
GCL_EXPECTED_FILE := Makefile
GCL_TARGET := $(LIBVMI_PATCH_STAMP)
GCL_DO_CHECK :=

include makelib/git.mk

$(call undefine-namespaces,LIBVMI)
