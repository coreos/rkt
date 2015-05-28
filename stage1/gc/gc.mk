include ../../makelib/lib.mk

.PHONY: build install

ISCRIPT := $(BUILDDIR)/install.d/10gc.install

install: build
	@echo $(call dep-install-file-to,$(GOBIN)/gc,/) > $(ISCRIPT)
	@echo $(call dep-install-file,$(call find-file-so-deps, $(GOBIN)/init))  >> $(ISCRIPT)

build:
	go install $(REPO_PATH)/stage1/gc
