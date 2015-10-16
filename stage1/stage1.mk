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
_STAGE1_SUBDIRS_ := diagexec prepare-app net-plugins net init gc reaper units aci usr_from_$(RKT_STAGE1_USR_FROM)
ifeq ($(RKT_STAGE1_USR_FROM),kvm)
_STAGE1_SUBDIRS_ += enter_kvm
else
_STAGE1_SUBDIRS_ += enter
endif

_STAGE1_ACI_ := $(BINDIR)/stage1-$(RKT_STAGE1_USR_FROM).aci
STAGE1_COPY_SO_DEPS :=
STAGE1_INSTALL_FILES :=
STAGE1_INSTALL_SYMLINKS :=
STAGE1_INSTALL_DIRS :=
STAGE1_CREATE_DIRS :=

$(call setup-stamp-file,_STAGE1_ASSEMBLE_ACI_STAMP_,/assemble_aci)

$(call inc-many,$(foreach sd,$(_STAGE1_SUBDIRS_),$(sd)/$(sd).mk))

# var name,whether it has to be split,if yes which word is the created
# file
_STAGE1_FILE_VARS_ := \
	INSTALL_FILES:y:2 \
	INSTALL_SYMLINKS:y:2 \
	INSTALL_DIRS:y:1 \
	CREATE_DIRS:n

define _STAGE1_GEN_DEP_
$1: $$(STAGE1_USR_STAMPS)
endef

# append contents of STAGE1_INSTALL_FILES to INSTALL_FILES and
# generate dependencies of created files on $(STAGE1_USR_STAMPS)
$(foreach v,$(_STAGE1_FILE_VARS_), \
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

CLEAN_FILES += $(_STAGE1_ACI_)
TOPLEVEL_STAMPS += $(_STAGE1_ASSEMBLE_ACI_STAMP_)

$(_STAGE1_ASSEMBLE_ACI_STAMP_): $(_STAGE1_ACI_)
	touch "$@"

ifeq ($(STAGE1_COPY_SO_DEPS),)

$(_STAGE1_ACI_): $(STAGE1_STAMPS)

else

$(call inc-one,find-so-deps.mk)

$(_STAGE1_ACI_): $(STAGE1_FSD_STAMP)

endif

$(call forward-vars,$(_STAGE1_ACI_), \
	ACTOOL ACIDIR)
$(_STAGE1_ACI_): $(ACTOOL_STAMP) | $(BINDIR)
	"$(ACTOOL)" build --owner-root --overwrite "$(ACIDIR)" "$@"

$(STAGE1_STAMPS): $(STAGE1_USR_STAMPS)

$(call undefine-namespaces,STAGE1 _STAGE1)
