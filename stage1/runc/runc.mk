##
# * Check out runc to BUILD/tmp/runc/src
# * Apply patches in stage1/runc/patches
# * Build runc via Makefile
# * Copy to aci
##

$(call setup-tmp-dir,RUNC_TMPDIR)

# TODO(cdc): github does not support cloning a specific hash, only
# a named reference. We want to use a known-stable commit of runc,
# which currently doesn't really exist.
# So, I've tagged it in a fork.
RUNC_GIT := git://github.com/squeed/runc.git
RUNC_GIT_COMMIT := docker-v1.13


RUNC_SRCDIR := $(RUNC_TMPDIR)
RUNC_BINARY := $(RUNC_TMPDIR)/runc
RUNC_PATCHESDIR := $(MK_SRCDIR)/patches
RUNC_PATCHES := $(abspath $(RUNC_PATCHESDIR)/*.patch)


CLEAN_DIRS += $(RUNC_TMPDIR)

$(call forward-vars,$(RUNC_SRCDIR),RUNC_SRCDIR)

$(call setup-stamp-file,RUNC_BUILD_STAMP,/runc-build)
$(call setup-stamp-file,RUNC_PATCH_STAMP,/patch)
$(call setup-stamp-file,RUNC_DEPS_STAMP,/deps)
#$(call setup-dep-file,RUNC_PATCHES_DEPMK)
#$(call setup-filelist-file,RUNC_PATCHES_FILELIST,/patches)


# The root dependency is $(RUNC_BINARY), which aci_install_bin claims.
# It also sets up the dependency $(RUNC_BINARY): $(RUNC_BUILD_STAMP)
$(RUNC_BINARY): $(RUNC_PATCH_STAMP) $(RUNC_BUILD_STAMP)

# RUNC_BUILD_STAMP: RUNC_PATCH_STAMP
$(call forward-vars,$(RUNC_BUILD_STAMP),RUNC_SRCDIR)
$(call generate-stamp-rule,$(RUNC_BUILD_STAMP),$(RUNC_PATCH_STAMP),, \
	$(call vb,vt,BUILD EXT,runc) \
	$$(MAKE) -C $(RUNC_SRCDIR) )

# Apply all the patches.
$(call forward-vars,$(RUNC_PATCH_STAMP),RUNC_SRCDIR RUNC_PATCHES)
$(call generate-stamp-rule,$(RUNC_PATCH_STAMP),$(RUNC_SRCDIR)/Makefile,, \
	shopt -s nullglob; \
	git -C $(RUNC_SRCDIR) reset --hard HEAD; \
	$(call vb,vt,PATCH,runc) \
	for p in $(RUNC_PATCHES); do \
		git -C $(RUNC_SRCDIR) apply < "$$$${p}"; \
	done)


GCL_REPOSITORY := $(RUNC_GIT)
GCL_DIRECTORY := $(RUNC_SRCDIR)
GCL_COMMITTISH := $(RUNC_GIT_COMMIT)
GCL_EXPECTED_FILE := Makefile
GCL_TARGET := $(RUNC_PATCH_STAMP)
GCL_DO_CHECK :=
include makelib/git.mk

AIB_FLAVORS := $(STAGE1_FLAVORS)
AIB_BINARY := $(RUNC_BINARY)
AIB_BUILD_STAMP := $(RUNC_BUILD_STAMP)
include stage1/makelib/aci_install_bin.mk

## Install runc-prepare binary
$(call inc-one,runc-prepare/runc-prepare.mk)

$(call undefine-namespaces,RUNC)
