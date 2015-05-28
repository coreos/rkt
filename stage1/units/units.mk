include ../../makelib/lib.mk

BINARY := $(BINDIR)/enter
SRC := enter.c
ISCRIPT := $(BUILDDIR)/install.d/40units.install
PWD := $(shell pwd)

.PHONY: install

install:
	$(call write-template,install.tpl.sh,{{INSTALL}}=$(call escape_space,$(INSTALL)) {{PWD}}=$(PWD),$(ISCRIPT))
