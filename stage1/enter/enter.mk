include ../../makelib/lib.mk

BINARY := $(BINDIR)/enter
SRC := enter.c
ISCRIPT := $(BUILDDIR)/install.d/10enter.install

.PHONY: install

install: $(BINARY)
	@echo $(call dep-install-file-to,$(BINARY),/) > $(ISCRIPT)

$(BINARY): $(SRC) enter.mk
	$(CC) $(CFLAGS) -o $@ $(SRC) -static -s

clean:
	rm -f $(BINARY)
