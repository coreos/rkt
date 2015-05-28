ISCRIPT := $(BUILDDIR)/install.d/30net.install
CONFDIR := /etc/rkt/net.d
PWD := $(shell pwd)

.PHONY: install

install:
	@echo $(call dep-install-dir,$(CONFDIR)) > $(ISCRIPT)
	@echo $(call dep-install-file-to,$(PWD)/conf/*.conf,$(CONFDIR)) >> $(ISCRIPT)
