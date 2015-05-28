include ../../makelib/lib.mk

ISCRIPT := $(BUILDDIR)/install.d/10passwd.install
PWD := $(shell pwd)

.PHONY: install

install:
	@echo $(call dep-install-dir,0755,/etc) > $(ISCRIPT)
	@echo $(call dep-install-file-to,$(PWD)/passwd,/etc) >> $(ISCRIPT)
	@echo $(call dep-install-file-to,$(PWD)/group,/etc) >> $(ISCRIPT)
