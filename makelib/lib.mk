# Copyright 2015 The rkt Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.


# find-so-deps scans the passed folder and returns a list of external *.so libraries
# that the binaries in this path are depending on
find-so-deps = $(shell cd $(1) && find -type f | xargs file | grep ELF | cut -f1 -d: | LD_LIBRARY_PATH=usr/lib xargs ldd | grep -v ^\\. | grep '/' | sed  -e 's/^[[:space:]]*//' -e 's/.*=> //' -e 's/ (0x[0-9a-f]*)$$//' | grep -v ^[^/] | sort -u)

# find-file-so-deps returns the list of external *.so libraries used by the binary
# passed as an argument
find-file-so-deps = $(shell LD_LIBRARY_PATH=usr/lib ldd $(1) | grep -v ^\\. | grep '/' | sed  -e 's/^[[:space:]]*//' -e 's/.*=> //' -e 's/ (0x[0-9a-f]*)$$//' | grep -v ^[^/] | sort -u)

# installs file to the same location in the root fs
dep-install-file = $(foreach file,$(1),$(INSTALL) -m $(shell stat -c "%a" $(file)) -D '$(file)' '$${ROOT}$(file); ')

# installs file to the target directory
dep-install-file-to = $(foreach file,$(1),$(INSTALL) -m $(shell stat -c "%a" $(file)) -t '$${ROOT}$(2)' '$(file); ')

# installs file to the target directory and sets permissions manually
dep-install-file-to-perm = $(foreach file,$(1),$(INSTALL) -m $(3) -t '$${ROOT}$(2)' '$(file); ')

# makes sure dir exists in rootfs
dep-install-dir = $(foreach dir,$(2),$(INSTALL) -m $(1) -d '$${ROOT}$(dir); ')

dep-copy-fs = $(foreach dir,$(1),cp -af '$(dir)/.' '$${ROOT}/; ')

dep-copy-dir = $(foreach dir,$(1),cp -af '$(dir)/.' '$${ROOT}$(2); ')

dep-copy-files = $(INSTALL) -m $(1) $(2) '$${ROOT}$(3); '

dep-symlink = ln -sf $(1) '$${ROOT}/$(2); '

dep-unlink = unlink '$${ROOT}/$(1); '

dep-write-file = '$(INSTALL) -d $${ROOT}$(dir $(2)); echo $(1) > $${ROOT}$(2); '

dep-flavor = 'ln -sf "$(1)" "$${ROOT}/flavor"; '

dep-systemd-version = 'echo $(1) > $${ROOT}/systemd-version; '

define dep-systemd
	'$(INSTALL) -d $${ROOT}$(USR_LIB_DIR)/systemd/system/$(2); ln -sf $(USR_LIB_DIR)/systemd/system/$(1) $${ROOT}$(USR_LIB_DIR)/systemd/system/$(2); '
endef

define make-dirs
$(foreach dir, $(1),$(shell mkdir -p $(dir)))
endef

define make-dir-symlink
$(shell if [ ! -d "$(2)" ]; then ln -s $(1) $(2); fi)
endef

# not-installed returns 0 if the library is not installed, empty string otherwise
define not-installed
$(filter $(words $(shell which $(1))),0)
endef

define expect-installed
$(if $(call not-installed,$(1)),$(error $(2)),)
endef

define write-template
$(call expect-installed,bash,"needs bash to be installed")
$(call expect-installed,sed,"needs sed to be installed")
sed $(foreach pattern,$(2),-e "s_$(subst =,_,$(pattern))_") $(1) > $(3)
endef

define go-find-packages-with-tests
$(shell cd $(GOPATH)/src/$(REPO_PATH); \
  GOPATH=$(GOPATH) $(GO) list -f '{{.ImportPath}} {{.TestGoFiles}}' $(1) | \
  grep --invert-match '\[\]' | \
  $(foreach dir,$(2),grep --invert-match $(dir) |) \
  awk '{ print $$1 }')
endef

define go-find-packages
$(shell cd $(GOPATH)/src/$(REPO_PATH); \
  GOPATH=$(GOPATH) $(GO) list -f '{{.ImportPath}} {{.GoFiles}}' $(1) | \
  grep --invert-match '\[\]' | \
  $(foreach dir,$(2),grep --invert-match $(dir) |) \
  awk '{ print $$1 }')
endef
