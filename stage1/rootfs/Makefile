_SUBDIRS=usr shim enter
SUBDIRS=$(_SUBDIRS) aggregate
export CFLAGS=-Wall -Os

.PHONY: test clean subdirs $(SUBDIRS)

subdirs: $(SUBDIRS)

$(SUBDIRS):
	$(MAKE) -C $@ $(MAKECMDGOALS)

clean: $(SUBDIRS)
	rm -Rf $(S1) $(S1TAR)

test: $(SUBDIRS)

aggregate: $(_SUBDIRS)
