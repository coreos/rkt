# This makefile assembles stage1 rootfs
#
# stage1.mk makefile is meant to be executed by parent makefile
#
# The current build process consists of two major steps
#
# 1. Build systemd either from the coreos binary image distro
#    or from source code
#
# 2. Install various services and binaries
#
# Every subdirectory has a corresponding makefile with name subdir.mk
# (naming is chosen to simplify development, otherwise it will be a sea of Makefile files)
#
# Every subdir makefile builds files to $(BINDIR) and generates a special file
# to $(BUILDDIR)/install.d/{priority}{subdir}.install
#
# After calling this stage1.mk the parent makefile executes all scripts in install.d
# and packages this all into the aci image
#

STAGE1_STAMPS :=
STAGE1_USR_STAMPS :=
STAGE1_SUBDIRS := diagexec prepare-app enter net-plugins net init gc reaper units aci usr_from_$(RKT_STAGE1_USR_FROM)
STAGE1_ACI := $(BINDIR)/stage1.aci
STAGE1_INSTALL_FILES :=
STAGE1_INSTALL_SYMLINKS :=
STAGE1_INSTALL_DIRS :=
STAGE1_CREATE_DIRS :=

$(call setup-stamp-file,STAGE1_ASSEMBLE_ACI_STAMP,/assemble_aci)
$(call setup-stamp-file,STAGE1_FIND_SO_DEPS_STAMP,/find_so_deps)

$(call inc-many,$(foreach sd,$(STAGE1_SUBDIRS),$(sd)/$(sd).mk))

# var name,whether it has to be split,if yes which word is the created
# file
STAGE1_FILE_VARS := \
	INSTALL_FILES:y:2 \
	INSTALL_SYMLINKS:y:2 \
	INSTALL_DIRS:y:1 \
	CREATE_DIRS:n

define _STAGE1_GEN_DEP_
$1: $$(STAGE1_USR_STAMPS)
endef

# append contents of STAGE1_INSTALL_FILES to INSTALL_FILES and
# generate dependencies of created files on $(STAGE1_USR_STAMPS)
$(foreach v,$(STAGE1_FILE_VARS), \
	$(eval _STAGE1_VAR_LIST_ := $(subst :, ,$v)) \
	$(eval _STAGE1_VAR_NAME_ := $(word 1,$(_STAGE1_VAR_LIST_))) \
	$(eval _STAGE1_DO_SPLIT_ := $(word 2,$(_STAGE1_VAR_LIST_))) \
	$(eval _STAGE1_WORD_IDX_ := $(word 3,$(_STAGE1_VAR_LIST_))) \
	$(eval _STAGE1_S1_VAR_NAME_ := STAGE1_$(_STAGE1_VAR_NAME_)) \
	$(eval $(_STAGE1_VAR_NAME_) += $($(_STAGE1_S1_VAR_NAME_))) \
	$(eval $(foreach i,$($(_STAGE1_S1_VAR_NAME_)), \
		$(if $(filter y,$(_STAGE1_DO_SPLIT_)), \
			$(eval $(call _STAGE1_GEN_DEP_,$(word $(_STAGE1_WORD_IDX_),$(subst :, ,$i)))), \
			$(eval $(call _STAGE1_GEN_DEP_,$i)) \
		) \
	)) \
	$(eval $(_STAGE1_S1_VAR_NAME_) :=) \
)

STAGE1_FILE_VARS :=
_STAGE1_GEN_DEP_ :=

CLEAN_FILES += $(STAGE1_ACI)
TOPLEVEL_STAMPS += $(STAGE1_ASSEMBLE_ACI_STAMP)

$(STAGE1_ASSEMBLE_ACI_STAMP): $(STAGE1_ACI)
	touch "$@"

$(STAGE1_ACI): ACIDIR := $(ACIDIR)
$(STAGE1_ACI): ACTOOL := $(ACTOOL)
$(STAGE1_ACI): $(STAGE1_FIND_SO_DEPS_STAMP) $(ACTOOL_STAMP) | $(BINDIR)
	"$(ACTOOL)" build --overwrite "$(ACIDIR)" "$@"

$(STAGE1_STAMPS): $(STAGE1_USR_STAMPS)

STAGE1_LIBDIRS := $(ACIROOTFSDIR)/lib:$(ACIROOTFSDIR)/usr/lib64:$(ACIROOTFSDIR)/usr/lib:$(ACIROOTFSDIR)/usr/lib64

ifneq ($(LD_LIBRARY_PATH),)

STAGE1_LIBDIRS := $(STAGE1_LIBDIRS):$(LD_LIBRARY_PATH)

endif

$(STAGE1_FIND_SO_DEPS_STAMP): ACIROOTFSDIR := $(ACIROOTFSDIR)
$(STAGE1_FIND_SO_DEPS_STAMP): STAGE1_LIBDIRS := $(STAGE1_LIBDIRS)
$(STAGE1_FIND_SO_DEPS_STAMP): INSTALL := $(INSTALL)
$(STAGE1_FIND_SO_DEPS_STAMP): $(STAGE1_STAMPS)
	set -e; \
	all_libs=$$(find "$(ACIROOTFSDIR)" -type f | xargs file | grep ELF | cut -f1 -d: | LD_LIBRARY_PATH="$(STAGE1_LIBDIRS)" xargs ldd | grep -v '^[^[:space:]]' | grep '/' | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*(0x[0-9a-fA-F]*)//' -e 's/.*=>[[:space:]]*//' | grep '^/' | sort -u); \
	for f in $${all_libs}; do \
		$(INSTALL) -D "$${f}" "$(ACIROOTFSDIR)$${f}"; \
	done; \
	touch "$@"

STAGE1_STAMPS :=
STAGE1_USR_STAMPS :=
STAGE1_SUBDIRS :=
STAGE1_ACI :=
STAGE1_ASSEMBLE_ACI_STAMP :=
STAGE1_FIND_SO_DEPS_STAMP :=
STAGE1_LIBDIRS :=
