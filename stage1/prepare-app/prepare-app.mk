include ../../makelib/lib.mk

BINARY := $(BINDIR)/prepare-app
SRC := prepare-app.c
ISCRIPT := $(BUILDDIR)/install.d/10prepare-app.install

.PHONY: install

install: $(BINARY)
	@echo $(call dep-install-file-to,$(BINARY),/) > $(ISCRIPT)

$(BINARY): $(SRC) prepare-app.mk
	$(CC) $(CFLAGS) -o $@ $(SRC) -static -s

clean:
	rm -f $(BINARY)
