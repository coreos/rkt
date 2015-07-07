# make "all" a default target
all:

# makelib/inc.mk must be included first!
include makelib/inc.mk
include makelib/file-ops-prolog.mk
include makelib/variables.mk
include makelib/misc.mk

SHELL := $(BASH_SHELL)
TOPLEVEL_STAMPS :=
TOPLEVEL_CHECK_STAMPS :=
TOPLEVEL_SUBDIRS := rkt tests

ifneq ($(RKT_STAGE1_USR_FROM),none)
TOPLEVEL_SUBDIRS += stage1
endif

$(call inc-one,actool.mk)
$(call inc-many,$(foreach sd,$(TOPLEVEL_SUBDIRS),$(sd)/$(sd).mk))

all: $(TOPLEVEL_STAMPS)

$(TOPLEVEL_CHECK_STAMPS): $(TOPLEVEL_STAMPS)

.INTERMEDIATE: $(TOPLEVEL_CHECK_STAMPS)

check: $(TOPLEVEL_CHECK_STAMPS)

include makelib/file-ops-epilog.mk

.PHONY: all check
