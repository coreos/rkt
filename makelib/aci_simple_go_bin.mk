# inputs cleared after including this file:
# <none>

# inputs left alone:
# ACIROOTFSDIR - ACI rootfs directory
# TOOLSDIR - location for compiled binary
# STAMPSDIR - location for a stamp
# MK_SRCDIR - directory where the sources are
# MK_FILENAME - name of mk file including this file
# MK_PATH - path of mk file including this file

_ASGB_TMP_PATH_ ?= $(lastword $(MAKEFILE_LIST))
_ASGB_PATH_ := $(_ASGB_TMP_PATH_)

_ASGB_NAME_ := $(patsubst %.mk,%,$(MK_FILENAME))
_ASGB_ACI_BINARY_ := $(ACIROOTFSDIR)/$(_ASGB_NAME_)

$(call setup-stamp-file,_ASGB_STAMP_)

# variables for makelib/build_go_bin.mk
BGB_BINARY := $(TOOLSDIR)/$(_ASGB_NAME_)
BGB_PKG_IN_REPO := $(subst $(MK_TOPLEVEL_SRCDIR)/,,$(MK_SRCDIR))

STAGE1_INSTALL_FILES += $(BGB_BINARY):$(_ASGB_ACI_BINARY_):-
CLEAN_FILES += $(BGB_BINARY)
STAGE1_STAMPS += $(_ASGB_STAMP_)

$(BGB_BINARY): $(_ASGB_PATH_) | $(TOOLSDIR)
$(_ASGB_ACI_BINARY_) $(BGB_BINARY): $(MK_PATH)

$(_ASGB_STAMP_): $(_ASGB_ACI_BINARY_)
	touch "$@"

$(BGB_BINARY): $(_ASGB_PATH_)

include makelib/build_go_bin.mk

_ASGB_NAME_ :=
_ASGB_ACI_BINARY_ :=
_ASGB_STAMP_ :=
