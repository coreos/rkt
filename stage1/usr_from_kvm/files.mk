$(call setup-stamp-file,UFKF_STAMP)
UFKF_DIR := $(MK_SRCDIR)/files
UFKF_VAR_RUN := $(ACIROOTFSDIR)/var/run

UFKF_ACI_FILES := \
	$(ACIROOTFSDIR)/etc/passwd \
	$(ACIROOTFSDIR)/etc/shadow \
	$(ACIROOTFSDIR)/usr/lib64/systemd/system/sshd.socket \
	$(ACIROOTFSDIR)/etc/group \
	$(ACIROOTFSDIR)/etc/ssh/sshd_config \
	$(ACIROOTFSDIR)/usr/lib64/systemd/system/sshd-prep.service \
	$(ACIROOTFSDIR)/usr/lib64/systemd/system/sshd@.service

UFKF_SRC_FILES := $(addprefix $(UFKF_DIR)/,$(notdir $(UFKF_ACI_FILES)))

STAGE1_STAMPS += $(UFKF_STAMP)

STAGE1_INSTALL_FILES += $(join $(addsuffix :,$(UFKF_SRC_FILES)),$(addsuffix :0644,$(UFKF_ACI_FILES)))
STAGE1_INSTALL_DIRS += \
	$(addsuffix :0755,$(sort $(call to-dir,$(UFKF_ACI_FILES)))) \
	$(UFKF_VAR_RUN):0755

$(UFKF_STAMP): $(UFKF_ACI_FILES) | $(UFKF_VAR_RUN)
	touch "$@"

$(call undefine-namespaces,UFKF)
