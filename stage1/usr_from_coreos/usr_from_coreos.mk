UFC_SYSTEMD_VERSION := "v222"
UFC_IMG_RELEASE := "794.1.0"
UFC_IMG_URL := "http://alpha.release.core-os.net/amd64-usr/$(UFC_IMG_RELEASE)/coreos_production_pxe_image.cpio.gz"

$(call setup-tmp-dir,UFC_TMPDIR)

UFC_ROOTFS := $(UFC_TMPDIR)/rootfs
UFC_FILELIST := $(UFC_TMPDIR)/manifest.txt
UFC_MANIFESTS_DIR ?= $(MK_SRCDIR)/manifest.d
UFC_MANIFESTS := $(wildcard $(UFC_MANIFESTS_DIR)/*)

$(call setup-dep-file,UFC_DEPMK,manifests)
$(call setup-clean-file,UFC_ROOTFSDIR_CLEANMK,/rootfs)
$(call setup-clean-file,UFC_ACIROOTFSDIR_CLEANMK,/acirootfs)

UFC_PXE := $(UFC_TMPDIR)/pxe.img
UFC_SQUASHFS_BASE := usr.squashfs
UFC_SQUASHFS := $(UFC_TMPDIR)/$(UFC_SQUASHFS_BASE)
UFC_CACHE_SH := $(MK_SRCDIR)/cache.sh
UFC_STAMP := $(STAMPSDIR)/$(call path-to-stamp,$(MK_PATH))

$(call setup-stamp-file,UFC_STAMP)
$(call setup-stamp-file,UFC_MKBASE_STAMP,/mkbase)
$(call setup-stamp-file,UFC_ACI_ROOTFS_STAMP,/acirootfs)
$(call setup-stamp-file,UFC_ACIROOTFS_DEPS_STAMP,/acirootfs-deps)
$(call setup-stamp-file,UFC_ROOTFS_CLEAN_STAMP,/rootfs-clean)
$(call setup-stamp-file,UFC_ACIROOTFS_CLEAN_STAMP,/acirootfs-clean)

$(call setup-filelist-file,UFC_DETAILED_FILELIST)

ifneq ($(RKT_LOCAL_COREOS_PXE_IMAGE_PATH),)

UFC_PXE := $(abspath $(RKT_LOCAL_COREOS_PXE_IMAGE_PATH))

else

CLEAN_FILES += \
	$(UFC_PXE) \
	$(UFC_PXE).$(firstword $(shell echo -n $(UFC_IMG_URL) | md5sum)).sig

endif

ifneq ($(RKT_LOCAL_COREOS_PXE_IMAGE_SYSTEMD_VER),)

UFC_SYSTEMD_VERSION := $(RKT_LOCAL_COREOS_PXE_IMAGE_SYSTEMD_VER)

endif

INSTALL_DIRS += $(UFC_ITMP):-
STAGE1_USR_STAMPS += $(UFC_STAMP)
CLEAN_FILES += \
	$(UFC_FILELIST) \
	$(UFC_SQUASHFS) \
	$(ACIROOTFSDIR)/systemd-version
CLEAN_SYMLINKS += \
	$(ACIROOTFSDIR)/flavor \
	$(ACIROOTFSDIR)/lib64 \
	$(ACIROOTFSDIR)/lib \
	$(ACIROOTFSDIR)/bin
CLEAN_DIRS += \
	$(ACIROOTFSDIR)/proc \
	$(UFC_ROOTFS)

$(UFC_STAMP): $(UFC_ACI_ROOTFS_STAMP) $(UFC_ACIROOTFS_DEPS_STAMP) $(UFC_ACIROOTFS_CLEAN_STAMP) $(UFC_ROOTFS_CLEAN_STAMP)
	touch "$@"

$(call forward-vars,$(UFC_ACI_ROOTFS_STAMP), \
	ACIROOTFSDIR UFC_ROOTFS UFC_SYSTEMD_VERSION)
$(UFC_ACI_ROOTFS_STAMP): $(UFC_MKBASE_STAMP) $(UFC_FILELIST)
	set -e; \
	rm -rf "$(ACIROOTFSDIR)"; \
	cp -af "$(UFC_ROOTFS)/." "$(ACIROOTFSDIR)"; \
	 \
	ln -sf 'coreos' "$(ACIROOTFSDIR)/flavor"; \
	ln -sf 'usr/lib64' "$(ACIROOTFSDIR)/lib64"; \
	ln -sf 'usr/lib' "$(ACIROOTFSDIR)/lib"; \
	ln -sf 'usr/bin' "$(ACIROOTFSDIR)/bin"; \
	mkdir -p "$(ACIROOTFSDIR)/proc"; \
	echo "$(UFC_SYSTEMD_VERSION)" >"$(ACIROOTFSDIR)/systemd-version"; \
	touch "$@"

# This depmk can be created only when detailed filelist is generated
$(UFC_ACIROOTFS_DEPS_STAMP): $(UFC_DETAILED_FILELIST)
$(call generate-glob-deps,$(UFC_ACIROOTFS_DEPS_STAMP),$(UFC_ACI_ROOTFS_STAMP),$(UFC_DEPMK),,$(UFC_DETAILED_FILELIST),$(UFC_ROOTFS))

# This cleanmk can be created only when detailed filelist is generated
$(UFC_ACIROOTFS_CLEAN_STAMP): $(UFC_DETAILED_FILELIST)
$(call generate-clean-mk,$(UFC_ACIROOTFS_CLEAN_STAMP),$(UFC_ACIROOTFSDIR_CLEANMK),$(UFC_DETAILED_FILELIST),$(ACIROOTFSDIR))

$(call forward-vars,$(UFC_MKBASE_STAMP), \
	UFC_ROOTFS UFC_FILELIST UFC_SQUASHFS)
$(UFC_MKBASE_STAMP): $(UFC_SQUASHFS) $(UFC_FILELIST)
	set -e; \
	rm -rf "$(UFC_ROOTFS)"; \
	mkdir -p "$(UFC_ROOTFS)"; \
	unsquashfs -d "$(UFC_ROOTFS)/usr" -ef "$(UFC_FILELIST)" "$(UFC_SQUASHFS)"; \
	touch "$@"

# This filelist can be generated only after the pxe image was
# unpackaged and unsquashed
$(UFC_DETAILED_FILELIST): $(UFC_MKBASE_STAMP)
$(call generate-deep-filelist,$(UFC_DETAILED_FILELIST),$(UFC_ROOTFS))

# This cleanmk can be generated only after the detailed filelist was
# generated.
$(UFC_ROOTFS_CLEAN_STAMP): $(UFC_DETAILED_FILELIST)
$(call generate-clean-mk,$(UFC_ROOTFS_CLEAN_STAMP),$(UFC_ROOTFSDIR_CLEANMK),$(UFC_DETAILED_FILELIST),$(UFC_ROOTFS))

$(call forward-vars,$(UFC_SQUASHFS), \
	UFC_TMPDIR UFC_PXE UFC_SQUASHFS_BASE)
$(UFC_SQUASHFS): $(UFC_PXE) | $(UFC_TMPDIR)
	cd "$(UFC_TMPDIR)" && gzip -cd "$(UFC_PXE)" | cpio --unconditional --extract "$(UFC_SQUASHFS_BASE)"

$(UFC_FILELIST): $(UFC_MANIFESTS) | $(UFC_TMPDIR)
	cat $^ | sort -u > "$@.tmp"
	cmp "$@.tmp" "$@" || mv "$@.tmp" "$@"
	rm -f "$@.tmp"

ifeq ($(RKT_LOCAL_COREOS_PXE_IMAGE_PATH),)

$(call forward-vars,$(UFC_PXE), \
	UFC_TMPDIR UFC_IMG_URL BASH_SHELL UFC_CACHE_SH)
$(UFC_PXE): $(UFC_CACHE_SH) | $(UFC_TMPDIR)
	ITMP="$(UFC_TMPDIR)" IMG_URL="$(UFC_IMG_URL)" $(BASH_SHELL) $(UFC_CACHE_SH)

endif

$(call undefine-namespaces,UFC)
