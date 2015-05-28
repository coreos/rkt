include ../makelib/*.mk

.PHONY: all

$(call make-dir-symlink,$(BUILDDIR)/bin,../bin)

PWD := $(shell pwd)

ifneq ($(RKT_STAGE1_USR_FROM),none)
all:
	@echo [STAGE1] building prerequisites for stage1 tests...
	set -e ; ./build
	@echo [STAGE1] starting stage1 tests...
	sudo GOPATH=$(GOPATH) GOROOT=$(GOROOT) $(GO) test -v $(GO_TEST_FUNC_ARGS)
else
all:
	@echo [STAGE1] skiping stage 1 tests as configured by user
endif
