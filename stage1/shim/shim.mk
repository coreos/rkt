include ../../makelib/lib.mk

BINARY := $(BINDIR)/shim.so
SRC := shim.c
ISCRIPT := $(BUILDDIR)/install.d/10shim.install

.PHONY: install clean

install: $(BINARY)
	@echo $(call dep-install-file-to,$(BINARY),/) > $(ISCRIPT)
	@echo $(call dep-symlink,shim.so,fakesdboot.so) >> $(ISCRIPT)

$(BINARY): $(SRC) shim.mk
	$(CC) $(CFLAGS) $(SRC) -o $@ -shared -fPIC -Wl,--no-as-needed -ldl -lc

clean:
	rm -f $(BINARY)
