UFC_SYSTEMD_VERSION := "v220"
UFC_IMG_RELEASE := "709.0.0"
UFC_IMG_URL := "http://alpha.release.core-os.net/amd64-usr/$(UFC_IMG_RELEASE)/coreos_production_pxe_image.cpio.gz"

UFC_ITMP := $(BUILDDIR)/tmp/usr_from_coreos
UFC_ROOTFS := $(UFC_ITMP)/rootfs
UFC_FILELIST := $(UFC_ITMP)/manifest.txt
UFC_MANIFESTS := $(wildcard $(MK_SRCDIR)/manifest.d/*)

$(call setup-dep-file,UFC_DEPMK,manifests)

UFC_PXE := $(UFC_ITMP)/pxe.img
UFC_SQUASHFS_BASE := usr.squashfs
UFC_SQUASHFS := $(UFC_ITMP)/$(UFC_SQUASHFS_BASE)
UFC_CACHE_SH := $(MK_SRCDIR)/cache.sh
UFC_STAMP := $(STAMPSDIR)/$(call path-to-stamp,$(MK_PATH))

$(call setup-stamp-file,UFC_STAMP)
$(call setup-stamp-file,UFC_MKBASE_STAMP,/mkbase)
$(call setup-stamp-file,UFC_ACI_ROOTFS_STAMP,/aci_rootfs)

INSTALL_DIRS += $(UFC_ITMP):-
STAGE1_USR_STAMPS += $(UFC_STAMP)

$(UFC_STAMP): $(UFC_ACI_ROOTFS_STAMP)
	touch "$@"

-include $(UFC_DEPMK)
$(UFC_ACI_ROOTFS_STAMP): UFC_ROOTFS := $(UFC_ROOTFS)
$(UFC_ACI_ROOTFS_STAMP): ACIROOTFSDIR := $(ACIROOTFSDIR)
$(UFC_ACI_ROOTFS_STAMP): UFC_FILELIST := $(UFC_FILELIST)
$(UFC_ACI_ROOTFS_STAMP): UFC_DEPMK := $(UFC_DEPMK)
$(UFC_ACI_ROOTFS_STAMP): $(UFC_MKBASE_STAMP) $(UFC_FILELIST)
	set -e; \
	cp -af "$(UFC_ROOTFS)/." "$(ACIROOTFSDIR)"; \
	 \
	truncate --size=0 "$(UFC_DEPMK).tmp"; \
	for l in $$(cat "$(UFC_FILELIST)"); do \
		fl="$(ACIROOTFSDIR)/usr/$${l}"; \
		ft=$$(stat --format="%F" "$${fl}"); \
		if [[ "$${ft}" = 'regular file' ]]; then \
			echo '$$(UFC_ACI_ROOTFS_STAMP): '"$${fl}" >>"$(UFC_DEPMK).tmp"; \
		fi; \
	done; \
	mv "$(UFC_DEPMK).tmp" "$(UFC_DEPMK)"; \
	 \
	ln -sf 'coreos' "$(ACIROOTFSDIR)/flavor"; \
	echo "$(UFC_SYSTEMD_VERSION)" >"$(ACIROOTFSDIR)/systemd-version"; \
	touch "$@"

$(UFC_MKBASE_STAMP): UFC_SQUASHFS := $(UFC_SQUASHFS)
$(UFC_MKBASE_STAMP): UFC_FILELIST := $(UFC_FILELIST)
$(UFC_MKBASE_STAMP): UFC_ROOTFS := $(UFC_ROOTFS)
$(UFC_MKBASE_STAMP): $(UFC_SQUASHFS) $(UFC_FILELIST)
	rm -rf "$(UFC_ROOTFS)"
	mkdir -p "$(UFC_ROOTFS)"
	unsquashfs -d "$(UFC_ROOTFS)/usr" -ef "$(UFC_FILELIST)" "$(UFC_SQUASHFS)"
	touch "$@"

$(UFC_SQUASHFS): UFC_SQUASHFS_BASE := $(UFC_SQUASHFS_BASE)
$(UFC_SQUASHFS): UFC_PXE := $(UFC_PXE)
$(UFC_SQUASHFS): UFC_ITMP := $(UFC_ITMP)
$(UFC_SQUASHFS): $(UFC_PXE)
	cd "$(UFC_ITMP)" && gzip -cd "$(UFC_PXE)" | cpio --unconditional --extract "$(UFC_SQUASHFS_BASE)"

$(UFC_FILELIST): $(UFC_MANIFESTS) | $(UFC_ITMP)
	cat $^ | sort -u > "$@.tmp"
	cmp "$@.tmp" "$@" || mv "$@.tmp" "$@"
	rm -f "$@.tmp"

$(UFC_PXE): UFC_CACHE_SH := $(UFC_CACHE_SH)
$(UFC_PXE): UFC_ITMP := $(UFC_ITMP)
$(UFC_PXE): UFC_IMG_URL := $(UFC_IMG_URL)
$(UFC_PXE): $(UFC_CACHE_SH) | $(UFC_ITMP)
	ITMP="$(UFC_ITMP)" IMG_URL="$(UFC_IMG_URL)" $(BASH_SHELL) $(UFC_CACHE_SH)

UFC_ITMP :=
UFC_ROOTFS :=
UFC_FILELIST :=
UFC_MANIFESTS :=
UFC_DEPMK :=
UFC_PXE :=
UFC_SQUASHFS_BASE :=
UFC_SQUASHFS :=
UFC_CACHE_SH :=
UFC_STAMP :=
UFC_MKBASE_STAMP :=
UFC_ACI_ROOTFS_STAMP :=
