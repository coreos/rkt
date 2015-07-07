LOCAL_NAME := $(patsubst %.mk,%,$(MK_FILENAME))

$(call setup-stamp-file,RKT_STAMP)

RKT_BINARY := $(BINDIR)/$(LOCAL_NAME)

# variables for makelib/build_go.mk
BGB_BINARY := $(RKT_BINARY)
BGB_PKG_IN_REPO := $(subst $(MK_TOPLEVEL_SRCDIR)/,,$(MK_SRCDIR))
BGB_GO_FLAGS := $(RKT_STAGE1_IMAGE_FLAGS) $(RKT_TAGS)

CLEAN_FILES += $(BGB_BINARY)
TOPLEVEL_STAMPS += $(RKT_STAMP)

$(RKT_STAMP): $(BGB_BINARY)
	touch "$@"

$(BGB_BINARY): $(MK_PATH) | $(BINDIR)

include makelib/build_go_bin.mk

LOCAL_NAME :=
# RKT_STAMP deliberately not cleared
# RKT_BINARY deliberately not cleared
