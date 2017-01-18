$(call setup-stamp-file,SKIM_ACI_STAMP,aci-manifest)
$(call setup-tmp-dir,SKIM_ACI_TMPDIR_BASE)

SKIM_ACI_TMPDIR := $(SKIM_ACI_TMPDIR_BASE)/skim
# a manifest template
SKIM_ACI_SRC_MANIFEST := $(MK_SRCDIR)/aci-manifest.in
# generated manifest to be copied to the ACI directory
SKIM_ACI_GEN_MANIFEST := $(SKIM_ACI_TMPDIR)/manifest
# manifest in the ACI directory
SKIM_ACI_MANIFEST := $(SKIM_ACIDIR)/manifest
# escaped values of the ACI name, version and enter command, so
# they can be safely used in the replacement part of sed's s///
# command.
SKIM_ACI_VERSION := $(call sed-replacement-escape,$(RKT_VERSION))
SKIM_ACI_ARCH := $(call sed-replacement-escape,$(RKT_ACI_ARCH))
# stamp and dep file for invalidating the generated manifest if name,
# version or enter command changes for this flavor
$(call setup-stamp-file,SKIM_ACI_MANIFEST_KV_DEPMK_STAMP,$manifest-kv-dep)
$(call setup-dep-file,SKIM_ACI_MANIFEST_KV_DEPMK,manifest-kv-dep)
SKIM_ACI_DIRS := \
	$(SKIM_ACIROOTFSDIR)/rkt \
	$(SKIM_ACIROOTFSDIR)/rkt/status \
	$(SKIM_ACIROOTFSDIR)/opt \
	$(SKIM_ACIROOTFSDIR)/opt/stage2

# main stamp rule - makes sure manifest and deps files are generated
$(call generate-stamp-rule,$(SKIM_ACI_STAMP),$(SKIM_ACI_MANIFEST) $(SKIM_ACI_MANIFEST_KV_DEPMK_STAMP))

# invalidate generated manifest if version or arch changes
$(call generate-kv-deps,$(SKIM_ACI_MANIFEST_KV_DEPMK_STAMP),$(SKIM_ACI_GEN_MANIFEST),$(SKIM_ACI_MANIFEST_KV_DEPMK),SKIM_ACI_VERSION SKIM_ACI_ARCH)

# this rule generates a manifest
$(call forward-vars,$(SKIM_ACI_GEN_MANIFEST), \
	SKIM_ACI_VERSION SKIM_ACI_ARCH)
$(SKIM_ACI_GEN_MANIFEST): $(SKIM_ACI_SRC_MANIFEST) | $(SKIM_ACI_TMPDIR) $(SKIM_ACI_DIRS) $(SKIM_ACIROOTFSDIR)/flavor
	$(VQ) \
	set -e; \
	$(call vb,vt,MANIFEST,skim) \
	sed \
		-e 's/@RKT_STAGE1_VERSION@/$(SKIM_ACI_VERSION)/g' \
    -e 's/@RKT_STAGE1_ARCH@/$(SKIM_ACI_ARCH)/g' \
	"$<" >"$@.tmp"; \
	$(call bash-cond-rename,$@.tmp,$@)

INSTALL_DIRS += \
	$(SKIM_ACI_TMPDIR):- \
	$(foreach d,$(SKIM_ACI_DIRS),$d:-)
INSTALL_SYMLINKS += \
	skim:$(SKIM_ACIROOTFSDIR)/flavor
SKIM_STAMPS += $(SKIM_ACI_STAMP)
INSTALL_FILES += \
	$(SKIM_ACI_GEN_MANIFEST):$(SKIM_ACI_MANIFEST):0644
CLEAN_FILES += $(SKIM_ACI_GEN_MANIFEST)

$(call undefine-namespaces,SKIM_ACI _SKIM_ACI)
