# inputs cleared after including this file:
# ASSCB_EXTRA_HEADERS - headers used

# inputs left alone:
# ACIROOTFSDIR - ACI rootfs directory
# TOOLSDIR - location for compiled binary
# STAMPSDIR - location for a stamp
# MK_SRCDIR - directory where the sources are
# MK_FILENAME - name of mk file including this file
# MK_PATH - path of mk file including this file

_ASSCB_TMP_PATH_ ?= $(lastword $(MAKEFILE_LIST))
_ASSCB_PATH_ := $(_ASSCB_TMP_PATH_)

_ASSCB_NAME_ := $(patsubst %.mk,%,$(MK_FILENAME))
_ASSCB_ACI_BINARY_ := $(ACIROOTFSDIR)/$(_ASSCB_NAME_)

$(call setup-stamp-file,_ASSCB_STAMP_)

# variables for build_static_c_bin.mk
BSCB_BINARY := $(TOOLSDIR)/$(_ASSCB_NAME_)
BSCB_HEADERS := $(foreach h,$(ASSCB_EXTRA_HEADERS),$(MK_SRCDIR)/$h)
BSCB_SOURCES := $(MK_SRCDIR)/$(_ASSCB_NAME_).c
BSCB_ADDITIONAL_CFLAGS := -Wall -Os

CLEAN_FILES += $(BSCB_BINARY) $(_ASSCB_ACI_BINARY_)
STAGE1_STAMPS += $(_ASSCB_STAMP_)
STAGE1_INSTALL_FILES += $(BSCB_BINARY):$(_ASSCB_ACI_BINARY_):-

$(BSCB_BINARY): $(_ASSCB_PATH_) | $(TOOLSDIR)
$(_ASSCB_ACI_BINARY_) $(BSCB_BINARY): $(MK_PATH)

$(_ASSCB_STAMP_): $(_ASSCB_ACI_BINARY_)
	touch "$@"

include makelib/build_static_c_bin.mk

ASSCB_EXTRA_HEADERS :=
_ASSCB_NAME_ :=
_ASSCB_ACI_BINARY_ :=
_ASSCB_STAMP_ :=
