LOCAL_PLUGIN_NAMES := main/veth main/bridge main/macvlan ipam/host-local
# note that it is relative path
LOCAL_ACICHROOT_PLUGINSDIR := usr/lib/rkt/plugins/net
LOCAL_ACI_PLUGINSDIR := $(ACIROOTFSDIR)/$(LOCAL_ACICHROOT_PLUGINSDIR)
LOCAL_HOST_LOCAL_PTP_SYMLINK := $(ACIROOTFSDIR)/host-local-ptp

$(call setup-stamp-file,LOCAL_STAMP)

define local-to-aci-plugin
$(LOCAL_ACI_PLUGINSDIR)/$(notdir $1)
endef

define local-to-local-plugin
$(TOOLSDIR)/$(notdir $1)
endef

LOCAL_PLUGINS :=
LOCAL_ACI_PLUGINS :=
LOCAL_PLUGIN_INSTALL_TRIPLETS :=
$(foreach p,$(LOCAL_PLUGIN_NAMES), \
        $(eval _NET_PLUGINS_MK_LOCAL_PLUGIN_ := $(call local-to-local-plugin,$p)) \
        $(eval _NET_PLUGINS_MK_ACI_PLUGIN_ := $(call local-to-aci-plugin,$p)) \
        $(eval LOCAL_PLUGINS += $(_NET_PLUGINS_MK_LOCAL_PLUGIN_)) \
        $(eval LOCAL_ACI_PLUGINS += $(_NET_PLUGINS_MK_ACI_PLUGIN_)) \
        $(eval LOCAL_PLUGIN_INSTALL_TRIPLETS += $(_NET_PLUGINS_MK_LOCAL_PLUGIN_):$(_NET_PLUGINS_MK_ACI_PLUGIN_):-) \
        $(eval _NET_PLUGINS_MK_LOCAL_PLUGIN_ :=) \
        $(eval _NET_PLUGINS_MK_ACI_PLUGIN_ :=))

$(LOCAL_STAMP): $(LOCAL_ACI_PLUGINS) | $(LOCAL_HOST_LOCAL_PTP_SYMLINK)
	touch "$@"

STAGE1_INSTALL_DIRS += $(LOCAL_ACI_PLUGINSDIR):0755
STAGE1_INSTALL_FILES += $(LOCAL_PLUGIN_INSTALL_TRIPLETS)
STAGE1_INSTALL_SYMLINKS += $(LOCAL_ACICHROOT_PLUGINSDIR)/host-local:$(LOCAL_HOST_LOCAL_PTP_SYMLINK)
CLEAN_FILES += $(LOCAL_PLUGINS)
STAGE1_STAMPS += $(LOCAL_STAMP)

define local-build-plugin-rule
BGB_BINARY := $$(call local-to-local-plugin,$1)
BGB_PKG_IN_REPO := Godeps/_workspace/src/github.com/appc/cni/plugins/$1
$$(BGB_BINARY): | $$(TOOLSDIR)
include makelib/build_go_bin.mk
endef

$(foreach p,$(LOCAL_PLUGIN_NAMES), \
        $(eval $(call local-build-plugin-rule,$p)))

LOCAL_PLUGIN_NAMES :=
LOCAL_ACICHROOT_PLUGINSDIR :=
LOCAL_ACI_PLUGINSDIR :=
LOCAL_HOST_LOCAL_PTP_SYMLINK :=
LOCAL_STAMP :=

local-to-aci-plugin :=
local-to-local-plugin :=

LOCAL_PLUGINS :=
LOCAL_ACI_PLUGINS :=
LOCAL_PLUGIN_INSTALL_TRIPLETS :=

local-build-plugin-rule :=
