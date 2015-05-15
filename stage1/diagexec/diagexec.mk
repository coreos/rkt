include ../../makelib/lib.mk

BINARY=$(BINDIR)/diagexec
SRC=diagexec.c
ISCRIPT := $(BUILDDIR)/install.d/10diagexec.install

.PHONY: clean install

install: $(BINARY)
	@echo $(call dep-install-file-to,$(BINARY),/) > $(ISCRIPT)

$(BINARY): $(SRC) elf.h diagexec.mk
	$(CC) $(CFLAGS) -o $@ $(SRC) -static -s

clean:
	rm -f $(BINARY)
