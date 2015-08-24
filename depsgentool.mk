$(call setup-stamp-file,DEPSGENTOOL_STAMP)

# variables for makelib/build_go_bin.mk
BGB_STAMP := $(DEPSGENTOOL_STAMP)
BGB_PKG_IN_REPO := tools/depsgen
BGB_BINARY := $(DEPSGENTOOL)
BGB_ADDITIONAL_GO_ENV := GOARCH=$(GOARCH_FOR_BUILD)

CLEAN_FILES += $(DEPSGENTOOL)

$(DEPSGENTOOL_STAMP):
	touch "$@"

$(DEPSGENTOOL): $(MK_PATH) | $(TOOLSDIR)

include makelib/build_go_bin.mk

# DEPSGENTOOL_STAMP deliberately not cleared
