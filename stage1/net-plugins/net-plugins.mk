include ../../makelib/lib.mk

ISCRIPT := $(BUILDDIR)/install.d/30net-plugins.install
PLUGINS := main/veth main/bridge main/macvlan ipam/host-local
PLUGINSDIR := /usr/lib/rkt/plugins/net

.PHONY: install $(PLUGINS)

install: $(PLUGINS)
	@echo $(call dep-install-dir,755,$(PLUGINSDIR)) > $(ISCRIPT)
	@echo $(call dep-install-file-to,$(foreach p,$(PLUGINS),$(BINDIR)/$(notdir $(p))),$(PLUGINSDIR)) >> $(ISCRIPT)
	@echo $(call dep-symlink,$(PLUGINSDIR)/host-local,host-local-ptp) >> $(ISCRIPT)

$(PLUGINS):
	go build -o $(BINDIR)/$(notdir $@) $(REPO_PATH)/Godeps/_workspace/src/github.com/appc/cni/plugins/$@
