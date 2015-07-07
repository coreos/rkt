# Creates a rule creating directories.
# Example: $(eval $(call _FILE_OPS_MK_CREATE_DIRS_RULE_,dir1 dir1/dir2 dir1/dir2/dir3))
define _FILE_OPS_MK_CREATE_DIRS_RULE_
CLEAN_DIRS += $1
$1:
	[[ -e "$$@" ]] || mkdir -p "$$@"
	$$(call _FILE_OPS_MK_BAIL_OUT_IF_NOT_DIR_,$$@)
endef

# Creates a rule for installing directory. Depends on parent
# directory.
# Example: $(eval $(call _FILE_OPS_MK_CREATE_INSTALL_DIR_RULE_,dir:0644))
define _FILE_OPS_MK_CREATE_INSTALL_DIR_RULE_
$$(eval $$(call _FILE_OPS_MK_SPLIT_2_,$1,_FILE_OPS_MK_DIR_,_FILE_OPS_MK_MODE_))
CLEAN_DIRS += $$(_FILE_OPS_MK_DIR_)
$$(_FILE_OPS_MK_DIR_): INSTALL := $$(INSTALL)
$$(_FILE_OPS_MK_DIR_): _FILE_OPS_MK_MODE_ := $$(_FILE_OPS_MK_MODE_)
#$$(_FILE_OPS_MK_DIR_): | $$(call to-dir,$$(_FILE_OPS_MK_DIR_))
$$(_FILE_OPS_MK_DIR_):
	[[ -e "$$@" ]] || $$(INSTALL) $$(call _FILE_OPS_MK_DASH_M_,$$(_FILE_OPS_MK_MODE_)) -d "$$@"
	$$(call _FILE_OPS_MK_BAIL_OUT_IF_NOT_DIR_,$$@)
$$(eval _FILE_OPS_MK_DIR_ :=)
$$(eval _FILE_OPS_MK_MODE_ :=)
endef

# Creates a rule for installing a file. Depends on source file and
# parent directory. Pass - as a third parameter for dest to inherit
# mode from src.
# Example: $(eval $(call _FILE_OPS_MK_CREATE_INSTALL_FILE_RULE_,src,dest,0755))
define _FILE_OPS_MK_CREATE_INSTALL_FILE_RULE_
$$(eval $$(call _FILE_OPS_MK_SPLIT_3_,$1,_FILE_OPS_MK_SRC_,_FILE_OPS_MK_DEST_,_FILE_OPS_MK_MODE_))
CLEAN_FILES += $$(_FILE_OPS_MK_DEST_)
$$(_FILE_OPS_MK_DEST_): INSTALL := $$(INSTALL)
$$(_FILE_OPS_MK_DEST_): _FILE_OPS_MK_MODE_ := $$(_FILE_OPS_MK_MODE_)
$$(_FILE_OPS_MK_DEST_): $$(_FILE_OPS_MK_SRC_) | $$(call to-dir,$$(_FILE_OPS_MK_DEST_))
	$$(INSTALL) $$(call _FILE_OPS_MK_DASH_M_,$$(_FILE_OPS_MK_MODE_)) "$$<" "$$@"
$$(eval _FILE_OPS_MK_SRC_ :=)
$$(eval _FILE_OPS_MK_DEST_ :=)
$$(eval _FILE_OPS_MK_MODE_ :=)
endef

# Creates a rule for installing a symlink. Depends on parent
# directory.
# Example: $(eval $(call _FILE_OPS_MK_CREATE_INSTALL_FILE_RULE_,src,dest,0755))
define _FILE_OPS_MK_CREATE_INSTALL_SYMLINK_RULE_
$$(eval $$(call _FILE_OPS_MK_SPLIT_2_,$1,_FILE_OPS_MK_TARGET_,_FILE_OPS_MK_LINK_NAME_))
CLEAN_FILES += $$(_FILE_OPS_MK_LINK_NAME_)
$$(_FILE_OPS_MK_LINK_NAME_): _FILE_OPS_MK_TARGET_ := $$(_FILE_OPS_MK_TARGET_)
$$(_FILE_OPS_MK_LINK_NAME_): | $$(call to-dir,$$(_FILE_OPS_MK_LINK_NAME_))
	set -e; \
	if [ -h "$$@" ]; then \
		tgt=$$$$(readlink "$$@"); \
		if [ "$$$${tgt}" != "$$(_FILE_OPS_MK_TARGET_)" ]; then \
			echo "'$$@' is a symlink pointing to '$$$${tgt}' instead of '$$(_FILE_OPS_MK_TARGET_)', bailing out" >&2; \
			exit 1; \
		fi; \
	elif [ -e "$$@" ]; then \
		echo "$$@ already exists and is not a symlink, bailing out" >&2; \
		exit 1; \
	else \
		ln -s "$$(_FILE_OPS_MK_TARGET_)" "$$@"; \
	fi
$$(eval _FILE_OPS_MK_TARGET_ :=)
$$(eval _FILE_OPS_MK_LINK_NAME_ :=)
endef

# Print an error if name is not a directory. To be used inside rules.
# Example $(call _FILE_OPS_MK_BAIL_OUT_IF_NOT_DIR_,dir)
define _FILE_OPS_MK_BAIL_OUT_IF_NOT_DIR_
if [[ ! -d "$1" ]]; then echo "$1 is not a directory, bailing out" >&2; exit 1; fi
endef

# Returns -m <foo> if foo is not a dash. Used for install invocations.
# Example: $(call _FILE_OPS_MK_DASH_M_:0755)
define _FILE_OPS_MK_DASH_M_
$(if $(filter-out -,$1),-m $1)
endef

define _FILE_OPS_MK_SPLIT_2_COMMON_
$(eval _FILE_OPS_MK_SPLITTED_ := $(subst :, ,$1)) \
$(eval $2 := $(word 1,$(_FILE_OPS_MK_SPLITTED_))) \
$(eval $3 := $(word 2,$(_FILE_OPS_MK_SPLITTED_)))
endef

define _FILE_OPS_MK_SPLIT_2_
$(eval $(call _FILE_OPS_MK_SPLIT_2_COMMON_,$1,$2,$3)) \
$(eval _FILE_OPS_MK_SPLITTED_ :=)
endef

define _FILE_OPS_MK_SPLIT_3_
$(eval $(call _FILE_OPS_MK_SPLIT_2_COMMON_,$1,$2,$3)) \
$(eval $4 := $(word 3,$(_FILE_OPS_MK_SPLITTED_))) \
$(eval _FILE_OPS_MK_SPLITTED_ :=)
endef

# generate rule for mkdir
$(eval $(call _FILE_OPS_MK_CREATE_DIRS_RULE_,$(CREATE_DIRS)))

# generate rules for installing directories
$(foreach d,$(sort $(INSTALL_DIRS)), \
        $(eval $(call _FILE_OPS_MK_CREATE_INSTALL_DIR_RULE_,$d)))

# generate rules for installing files
$(foreach f,$(sort $(INSTALL_FILES)), \
        $(eval $(call _FILE_OPS_MK_CREATE_INSTALL_FILE_RULE_,$f)))

# generate rules for creating symlinks
$(foreach s,$(sort $(INSTALL_SYMLINKS)), \
        $(eval $(call _FILE_OPS_MK_CREATE_INSTALL_SYMLINK_RULE_,$s)))

_file_ops_mk_clean_:
	rm -f $(sort $(CLEAN_FILES))
	rmdir -p $(sort $(CLEAN_DIRS))

clean: _file_ops_mk_clean_

_FILE_OPS_MK_ALL_DIRS_ := \
	$(CREATE_DIRS) \
	$(foreach d,$(INSTALL_DIRS),$(firstword $(subst :, ,$d))) \
	$(foreach s,$(INSTALL_SYMLINKS),$(lastword $(subst :, ,$s)))

.PHONY: $(_FILE_OPS_MK_ALL_DIRS_) _file_ops_mk_clean_
