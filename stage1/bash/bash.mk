# grab bash from the host

include ../../makelib/lib.mk

ISCRIPT := $(BUILDDIR)/install.d/10bash.install

BASH_PATH := $(shell which $(BASH))

.PHONY: install

install:
	@echo $(call dep-install-file,$(BASH_PATH)) > $(ISCRIPT)
	@echo $(call dep-install-file,$(call find-file-so-deps,$(BASH_PATH))) >> $(ISCRIPT)
# some search it in /usr/bin too
	@echo $(call dep-install-dir,0755,/usr/bin) >> $(ISCRIPT)
	@echo $(call dep-install-file-to,$(BASH_PATH),/usr/bin) >> $(ISCRIPT)
