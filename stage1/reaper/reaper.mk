include ../../makelib/lib.mk

ISCRIPT := $(BUILDDIR)/install.d/10reaper.install
PWD := $(shell pwd)

.PHONY: install

install:
	@echo $(call dep-install-file-to-perm,$(PWD)/reaper.sh,/,755) > $(ISCRIPT)
