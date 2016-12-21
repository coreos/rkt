#Check out and build runc
$(call setup-tmp-dir,RUNC_TMPDIR)

RUNC_GIT := git://github.com/opencontainers/runc.git
# TODO(cdc) Change this to the next release
# Our git scaffolding doesn't support pinning to a commit, only a tag or branch
RUNC_GIT_COMMIT := v1.0.0-rc2


RUNC_SRCDIR := $(RUNC_TMPDIR)/src
RUNC_BINARY := $(RUNC_TMPDIR)/runc
RUNC_PATCHESDIR := $(MK_SRCDIR)/patches
RUNC_PATCHES := $(abspath $(RUNC_PATCHESDIR)/*.patch)


$(call setup-stamp-file,RUNC_STAMP)
$(call setup-stamp-file,RUNC_BUILD_STAMP,/build)
$(call setup-stamp-file,RUNC_CLONE_STAMP,/clone)
$(call setup-stamp-file,RUNC_DIR_CLEAN_STAMP,/dir-clean)
$(call setup-stamp-file,RUNC_DEPS_STAMP,/deps)
$(call setup-dep-file,RUNC_PATCHES_DEPMK)
$(call setup-filelist-file,RUNC_DIR_FILELIST,/dir)
$(call setup-clean-file,RUNC_CLEANMK,/src)

## Tie the dependencies together
$(call generate-stamp-rule,$(RUNC_STAMP),$(RUNC_BINARY) $(RUNC_DEPS_STAMP),,)

$(call forward-vars,$(RUNC_SRCDIR),RUNC_SRCDIR)
$(call forward-vars,$(RUNC_BINARY),RUNC_BINARY)
$(RUNC_BINARY): $(RUNC_BUILD_STAMP)
	cp $(RUNC_SRCDIR)/runc $(RUNC_BINARY);
 
$(call generate-stamp-rule,$(RUNC_BUILD_STAMP),$(RUNC_PATCH_STAMP) $(RUNC_CLONE_STAMP),, \
	$(call vb,vt,BUILD EXT,runc) \
	cd $(RUNC_SRCDIR); $$(MAKE) $(call vl2,--silent) $(call vl2,>/dev/null))

## Patches
# generate a filelist
$(call generate-patches-filelist,$(RUNC_PATCHES_FILELIST),$(RUNC_PATCHESDIR))
# generate a dep.mk on those patches
$(call generate-glob-deps,$(RUNC_DEPS_STAMP),$(RUNC_CLONE_STAMP),$(RUNC_PATCHES_DEPMK),.patch,$(RUNC_PATCHES_FILELIST),$(RUNC_PATCHESDIR),normal)


$(call generate-stamp-rule,$(RUNC_PATCH_STAMP),$(RUNC_CLONE_STAMP),, \
	shopt -s nullglob; \
	$(call vb,vt,PATCH,runc) \
	cd $(RUNC_SRCDIR); \
	for p in $(RUNC_PATCHES); do \
		echo $$$${p}; \
		git apply "$$$${p}"; \
	done)



$(RUNC_DIR_FILELIST): $(RUNC_CLONE_STAMP)
$(call generate-deep-filelist,$(RUNC_DIR_FILELIST),$(RUNC_SRCDIR))

$(call generate-clean-mk,$(RUNC_DIR_CLEAN_STAMP),$(RUNC_CLEANMK),$(RUNC_DIR_FILELIST),$(RUNC_SRCDIR))


GCL_REPOSITORY := $(RUNC_GIT)
GCL_DIRECTORY := $(RUNC_SRCDIR)
GCL_COMMITTISH := $(RUNC_GIT_COMMIT)
GCL_EXPECTED_FILE := Makefile
GCL_TARGET := $(RUNC_CLONE_STAMP)
GCL_DO_CHECK :=
include makelib/git.mk

AIB_FLAVORS := $(STAGE1_FLAVORS)
AIB_BINARY := $(RUNC_BINARY)
include stage1/makelib/aci_install_bin.mk

#$(call undefine-namespaces,RUNC)


$(call inc-one,runc-prepare/runc-prepare.mk)
