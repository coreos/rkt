# inputs cleared after including this file:
# BSCB_BINARY: path of a built binary
# BSCB_SOURCES: sources used to build a binary
# BSCB_HEADERS: headers used to build a binary
# BSCB_ADDITIONAL_CFLAGS: additional CFLAGS passed to CC, just after CFLAGS

# misc inputs (usually provided by default):
# CC - C compiler
# CFLAGS - flags passed to CC.

_BSCB_TMP_PATH_ ?= $(lastword $(MAKEFILE_LIST))
_BSCB_PATH_ := $(_BSCB_TMP_PATH_)

$(BSCB_BINARY): CC := $(CC)
$(BSCB_BINARY): CFLAGS := $(CFLAGS)
$(BSCB_BINARY): BSCB_ADDITIONAL_CFLAGS := $(BSCB_ADDITIONAL_CFLAGS)
$(BSCB_BINARY): BSCB_SOURCES := $(BSCB_SOURCES)
$(BSCB_BINARY): $(BSCB_SOURCES) $(BSCB_HEADERS)
$(BSCB_BINARY): $(_BSCB_PATH_)
	$(CC) $(CFLAGS) $(BSCB_ADDITIONAL_CFLAGS) -o "$@" $(BSCB_SOURCES) -static -s

CLEAN_FILES += $(BSCB_BINARY)

BSCB_BINARY :=
BSCB_SOURCES :=
BSCB_HEADERS :=
BSCB_ADDITIONAL_CFLAGS :=
