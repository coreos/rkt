UFK_STAMPS :=
UFK_INCLUDES := \
	../usr_from_coreos/usr_from_coreos.mk \
	kernel.mk \
	files.mk \
	lkvm.mk
$(call setup-stamp-file,UFK_REPLACE_FLAVOR_STAMP)
$(call setup-tmp-dir,UFK_TMPDIR)

UFC_MANIFESTS_DIR := $(MK_SRCDIR)/manifest.d
$(call inc-many,$(UFK_INCLUDES))

STAGE1_STAMPS += $(UFK_REPLACE_FLAVOR_STAMP) $(UFK_STAMPS)

$(call forward-vars,$(UFK_REPLACE_FLAVOR_STAMP), \
	ACIROOTFSDIR)
$(UFK_REPLACE_FLAVOR_STAMP):
	rm -f "$(ACIROOTFSDIR)/flavor"
	ln -sf 'kvm' "$(ACIROOTFSDIR)/flavor"
	ln -sf 'enter_kvm' "$(ACIROOTFSDIR)/enter"
	touch "$@"

$(call undefine-namespaces,UFK)
