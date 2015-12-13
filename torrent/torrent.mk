$(call setup-stamp-file,TORRENT_STAMP)

# variables for makelib/build_go_bin.mk
BGB_STAMP := $(TORRENT_STAMP)
BGB_PKG_IN_REPO :=torrent
BGB_BINARY := $(BINDIR)/torrent
BGB_ADDITIONAL_GO_ENV := GOARCH=$(GOARCH_FOR_BUILD)

CLEAN_FILES += $(BINDIR)/torrent
TOPLEVEL_STAMPS += $(TORRENT_STAMP)

$(call generate-stamp-rule,$(TORRENT_STAMP))

$(BGB_BINARY): $(MK_PATH) | $(BINDIR)

include makelib/build_go_bin.mk

# CLEANGENTOOL_STAMP deliberately not cleared
