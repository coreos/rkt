_SUBDIRS=usr diagexec enter net-plugins net init capsule
SUBDIRS=$(_SUBDIRS) aggregate
export CFLAGS=-Wall -Os

.PHONY: test clean subdirs $(SUBDIRS)

subdirs: $(SUBDIRS)

$(SUBDIRS):
	$(MAKE) -C $@ $(MAKECMDGOALS)

clean: $(SUBDIRS)

test: $(SUBDIRS)

aggregate: $(_SUBDIRS)
