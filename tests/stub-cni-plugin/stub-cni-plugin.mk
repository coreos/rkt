# stamp building the stub cni plugin
$(call setup-stamp-file,FTST_SCP_STAMP,/scp)
FTST_SCP_BINARY := $(FTST_TMPDIR)/stub-cni-plugin

CLEAN_FILES += $(FTST_SCP_BINARY)

# variables for makelib/build_go_bin.mk
BGB_STAMP := $(FTST_SCP_STAMP)
BGB_BINARY := $(FTST_SCP_BINARY)
BGB_PKG_IN_REPO := $(call go-pkg-from-dir)

include makelib/build_go_bin.mk

# do not undefine the FTST_SCP_BINARY and FTST_SCP_STAMP variables, we
# will use them in functional.mk
$(call undefine-namespaces,FTST_SCP,FTST_SCP_BINARY FTST_SCP_STAMP)
