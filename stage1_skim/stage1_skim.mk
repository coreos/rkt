SKIM_ACIDIR := $(BUILDDIR)/aci-for-skim-flavor
SKIM_ACIROOTFSDIR := $(SKIM_ACIDIR)/rootfs
SKIM_TOOLSDIR := $(TARGET_TOOLSDIR)/skim
SKIM_STAMPS :=
SKIM_SUBDIRS := run gc enter aci
SKIM_STAGE1 := $(TARGET_BINDIR)/stage1-skim.aci

$(call setup-stamp-file,SKIM_STAMP,aci-build)

$(call inc-many,$(foreach sd,$(SKIM_SUBDIRS),$(sd)/$(sd).mk))

$(call inc-one,stop/stop.mk)

$(call generate-stamp-rule,$(SKIM_STAMP),$(SKIM_STAMPS) $(ACTOOL_STAMP),$(TARGET_BINDIR), \
	$(call vb,vt,ACTOOL,$(call vsp,$(SKIM_STAGE1))) \
	"$(ACTOOL)" build --overwrite --owner-root "$(SKIM_ACIDIR)" "$(SKIM_STAGE1)")

INSTALL_DIRS += \
	$(SKIM_TOOLSDIR):- \
	$(SKIM_ACIDIR):- \
	$(SKIM_ACIROOTFSDIR):-

SKIM_FLAVORS := $(call commas-to-spaces,$(RKT_STAGE1_FLAVORS))

CLEAN_FILES += $(SKIM_STAGE1)

ifneq ($(filter skim,$(SKIM_FLAVORS)),)

# actually build the skim stage1 only if requested

TOPLEVEL_STAMPS += $(SKIM_STAMP)

endif

$(call undefine-namespaces,SKIM _SKIM)
