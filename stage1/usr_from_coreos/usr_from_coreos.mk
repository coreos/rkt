include ../../makelib/lib.mk

.PHONY: install

ISCRIPT := $(BUILDDIR)/install.d/00usr.install
ITMP := $(BUILDDIR)/tmp/usr_from_coreos

$(call make-dirs,$(ITEMP))

install: $(BUILDDIR)/usr_from_coreos.done
	@echo $(call dep-copy-fs,$(ITMP)/rootfs) > $(ISCRIPT)
	@echo $(call dep-flavor,coreos) >> $(ISCRIPT)

$(BUILDDIR)/usr_from_coreos.done: usr_from_coreos.mk mkbase.sh $(ITMP)/pxe.img manifest.d/*
	@ITMP=${ITMP} ./mkbase.sh && touch $(BUILDDIR)/usr_from_coreos.done

$(ITMP)/pxe.img: cache.sh
	@ITMP=${ITMP} ./cache.sh
