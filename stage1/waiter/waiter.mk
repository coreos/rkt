include ../../makelib/lib.mk

BINARY := $(BINDIR)/waiter
SRC := waiter.c
ISCRIPT := $(BUILDDIR)/install.d/10waiter.install

.PHONY: clean install

install: $(BINARY)
	@echo $(call dep-install-file-to,$(BINARY),/) > $(ISCRIPT)

$(BINARY): $(SRC) waiter.mk
	$(CC) $(CFLAGS) -o $@ $(SRC) -static -s

clean:
	rm -f $(BINARY)
