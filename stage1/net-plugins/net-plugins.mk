LOCAL_PLUGIN_NAMES := main/ptp main/bridge main/macvlan main/ipvlan ipam/host-local ipam/dhcp meta/flannel
LOCAL_ACI_PLUGINSDIR := $(ACIROOTFSDIR)/usr/lib/rkt/plugins/net
LOCAL_HOST_LOCAL_PTP_SYMLINK := $(LOCAL_ACI_PLUGINSDIR)/host-local-ptp

$(call setup-stamp-file,LOCAL_STAMP)

define LOCAL_NAME_TO_ACI_PLUGIN
$(LOCAL_ACI_PLUGINSDIR)/$(notdir $1)
endef

define LOCAL_NAME_TO_BUILT_PLUGIN
$(TOOLSDIR)/$(notdir $1)
endef

LOCAL_PLUGINS :=
LOCAL_ACI_PLUGINS :=
LOCAL_PLUGIN_INSTALL_TRIPLETS :=
$(foreach p,$(LOCAL_PLUGIN_NAMES), \
        $(eval _NET_PLUGINS_MK_LOCAL_PLUGIN_ := $(call LOCAL_NAME_TO_BUILT_PLUGIN,$p)) \
        $(eval _NET_PLUGINS_MK_ACI_PLUGIN_ := $(call LOCAL_NAME_TO_ACI_PLUGIN,$p)) \
        $(eval LOCAL_PLUGINS += $(_NET_PLUGINS_MK_LOCAL_PLUGIN_)) \
        $(eval LOCAL_ACI_PLUGINS += $(_NET_PLUGINS_MK_ACI_PLUGIN_)) \
        $(eval LOCAL_PLUGIN_INSTALL_TRIPLETS += $(_NET_PLUGINS_MK_LOCAL_PLUGIN_):$(_NET_PLUGINS_MK_ACI_PLUGIN_):-))

$(LOCAL_STAMP): $(LOCAL_ACI_PLUGINS) | $(LOCAL_HOST_LOCAL_PTP_SYMLINK)
	touch "$@"

STAGE1_INSTALL_DIRS += $(LOCAL_ACI_PLUGINSDIR):0755
STAGE1_INSTALL_FILES += $(LOCAL_PLUGIN_INSTALL_TRIPLETS)
STAGE1_INSTALL_SYMLINKS += host-local:$(LOCAL_HOST_LOCAL_PTP_SYMLINK)
CLEAN_FILES += $(LOCAL_PLUGINS)
STAGE1_STAMPS += $(LOCAL_STAMP)

define LOCAL_GENERATE_BUILD_PLUGIN_RULE
# variables for makelib/build_go_bin.mk
BGB_STAMP := $(LOCAL_STAMP)
BGB_BINARY := $$(call LOCAL_NAME_TO_BUILT_PLUGIN,$1)
BGB_PKG_IN_REPO := Godeps/_workspace/src/github.com/appc/cni/plugins/$1
$$(BGB_BINARY): | $$(TOOLSDIR)
include makelib/build_go_bin.mk
endef

$(foreach p,$(LOCAL_PLUGIN_NAMES), \
        $(eval $(call LOCAL_GENERATE_BUILD_PLUGIN_RULE,$p)))

$(call undefine-namespaces,LOCAL _NET_PLUGINS_MK)
