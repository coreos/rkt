# custom kernel compilation
KERNEL_VERSION := 4.1.3
KERNEL_TMP := $(BUILDDIR)/tmp/usr_from_kvm/kernel
KERNEL_NAME := linux-$(KERNEL_VERSION)
KERNEL_TARBALL := $(KERNEL_NAME).tar.xz
KERNEL_TARGET_FILE := $(KERNEL_TMP)/$(KERNEL_TARBALL)
KERNEL_SRCDIR := $(KERNEL_TMP)/$(KERNEL_NAME)
KERNEL_BUILDDIR := $(abspath $(KERNEL_TMP)/build-$(KERNEL_VERSION))
KERNEL_URL := https://www.kernel.org/pub/linux/kernel/v4.x/$(KERNEL_TARBALL)
KERNEL_MAKEFILE := $(KERNEL_SRCDIR)/Makefile
KERNEL_STUFFDIR := $(MK_SRCDIR)/kernel
KERNEL_SRC_CONFIG := $(KERNEL_STUFFDIR)/cutdown-config
KERNEL_PATCHES := $(abspath $(KERNEL_STUFFDIR)/patches/*.patch)
KERNEL_BUILD_CONFIG := $(KERNEL_BUILDDIR)/.config
KERNEL_BZIMAGE := $(KERNEL_BUILDDIR)/arch/x86/boot/bzImage
KERNEL_ACI_BZIMAGE := $(ACIROOTFSDIR)/bzImage

$(call setup-stamp-file,KERNEL_STAMP,/build_kernel)
$(call setup-stamp-file,KERNEL_PATCH_STAMP,/patch_kernel)
$(call setup-stamp-file,KERNEL_DEPS_STAMP,/deps)
$(call setup-dep-file,KERNEL_PATCHES_DEPMK)

CREATE_DIRS += $(KERNEL_TMP) $(KERNEL_BUILDDIR)
INSTALL_FILES += \
	$(KERNEL_SRC_CONFIG):$(KERNEL_BUILD_CONFIG):- \
	$(KERNEL_BZIMAGE):$(KERNEL_ACI_BZIMAGE):-
UFK_STAMPS += $(KERNEL_STAMP)

$(KERNEL_STAMP): $(KERNEL_ACI_BZIMAGE) $(KERNEL_DEPS_STAMP)
	touch "$@"

$(call forward-vars,$(KERNEL_BZIMAGE), \
	MAKE KERNEL_SRCDIR KERNEL_BUILDDIR)
$(KERNEL_BZIMAGE): $(KERNEL_BUILD_CONFIG) $(KERNEL_PATCH_STAMP)
	$(MAKE) -C "$(KERNEL_SRCDIR)" O="$(KERNEL_BUILDDIR)" bzImage

$(call forward-vars,$(KERNEL_PATCH_STAMP), \
	KERNEL_PATCHES KERNEL_SRCDIR)
$(KERNEL_PATCH_STAMP): $(KERNEL_MAKEFILE)
	set -e; \
	shopt -s nullglob; \
	for p in $(KERNEL_PATCHES); do \
		patch --directory="$(KERNEL_SRCDIR)" --strip=1 --forward <"$${p}"; \
	done; \
	touch "$@"

$(call generate-glob-deps,$(KERNEL_DEPS_STAMP)$(KERNEL_MAKEFILE),$(KERNEL_PATCHES_DEPMK),.patch,$(KERNEL_PATCHES))

$(call forward-vars,$(KERNEL_MAKEFILE), \
	KERNEL_SRCDIR KERNEL_TMP)
$(KERNEL_MAKEFILE): $(KERNEL_TARGET_FILE)
	rm -rf "$(KERNEL_SRCDIR)"
	tar --extract --xz --touch --file="$<" --directory="$(KERNEL_TMP)"

$(call forward-vars,$(KERNEL_TARGET_FILE), \
	KERNEL_URL)
$(KERNEL_TARGET_FILE): | $(KERNEL_TMP)
	wget --tries=20 --output-document="$@" "$(KERNEL_URL)"

$(call undefine-namespaces,KERNEL)
