LOCAL_UNITDIR := $(ACIROOTFSDIR)/usr/lib/systemd/system
LOCAL_UNIT_DIRS := \
        $(LOCAL_UNITDIR) \
        $(ACIROOTFSDIR)/usr/lib/systemd/system/default.target.wants \
        $(ACIROOTFSDIR)/usr/lib/systemd/system/sockets.target.wants
LOCAL_UNIT_FILES := \
        $(MK_SRCDIR)/units/default.target \
        $(MK_SRCDIR)/units/exit-watcher.service \
        $(MK_SRCDIR)/units/local-fs.target \
        $(MK_SRCDIR)/units/reaper.service \
        $(MK_SRCDIR)/units/sockets.target \
        $(MK_SRCDIR)/units/halt.target \

$(call setup-stamp-file,LOCAL_STAMP)

local-to-aci-unit = $(LOCAL_UNITDIR)/$(notdir $1)

LOCAL_ACI_UNIT_FILES :=
LOCAL_INSTALL_TRIPLETS :=
$(foreach u,$(LOCAL_UNIT_FILES), \
        $(eval _UNITS_MK_ACI_UNIT_ := $(call local-to-aci-unit,$u)) \
        $(eval LOCAL_ACI_UNIT_FILES += $(_UNITS_MK_ACI_UNIT_)) \
        $(eval LOCAL_INSTALL_TRIPLETS += $u:$(_UNITS_MK_ACI_UNIT_):0644) \
        $(eval _UNITS_MK_ACI_UNIT_ :=))

$(LOCAL_STAMP): $(LOCAL_ACI_UNIT_FILES) | $(LOCAL_UNIT_DIRS)
	touch "$@"

STAGE1_INSTALL_FILES += $(LOCAL_INSTALL_TRIPLETS)
STAGE1_INSTALL_DIRS += $(foreach d,$(LOCAL_UNIT_DIRS),$d:0755)
STAGE1_STAMPS += $(LOCAL_STAMP)

LOCAL_UNITDIR :=
LOCAL_UNIT_DIRS :=
LOCAL_UNIT_FILES :=
LOCAL_STAMP :=

local-to-aci-unit :=

LOCAL_ACI_UNIT_FILES :=
LOCAL_INSTALL_TRIPLETS :=
