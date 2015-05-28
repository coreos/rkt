include ../../makelib/lib.mk

ISCRIPT := $(BUILDDIR)/install.d/40aci.install

.PHONY: install

install:
	$(call write-template,install.tpl.sh,{{INSTALL}}=$(call escape_space,$(INSTALL)) {{PWD}}=$(shell pwd),$(ISCRIPT))
