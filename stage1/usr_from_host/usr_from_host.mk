$(call setup-stamp-file,UFH_STAMP)

STAGE1_USR_STAMPS += $(UFH_STAMP)

$(UFH_STAMP): ACIROOTFSDIR := $(ACIROOTFSDIR)
$(UFH_STAMP): | $(ACIROOTFSDIR)
	ln -sf 'host' "$(ACIROOTFSDIR)/flavor"
	touch "$@"
