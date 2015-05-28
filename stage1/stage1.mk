# This makefile assembles stage1 rootfs
#
# stage1.mk makefile is meant to be executed by parent makefile
#
# The current build process consists of two major steps
#
# 1. Build systemd either from the coreos binary image distro
#    or from source code
#
# 2. Install various services and binaries
#
# Every subdirectory has a corresponding makefile with name subdir.mk
# (naming is chosen to simplify development, otherwise it will be a sea of Makefile files)
#
# Every subdir makefile builds files to $(BINDIR) and generates a special file
# to $(BUILDDIR)/install.d/{priority}{subdir}.install
#
# After calling this stage1.mk the parent makefile executes all scripts in install.d
# and packages this all into the aci image
#
SUBDIRS := waiter shim diagexec prepare-app enter net-plugins net init gc reaper units aci passwd

ifeq ($(RKT_STAGE1_USR_FROM),coreos)
SUBDIRS += usr_from_coreos
endif

ifeq ($(RKT_STAGE1_USR_FROM),src)
SUBDIRS += usr_from_src bash
endif

ifeq ($(RKT_STAGE1_DBUS),yes)
SUBDIRS += dbus
endif

.PHONY: $(SUBDIRS) all modules

export CFLAGS=-Wall -Os

all: SHELL := $(BASH)
all: modules
	set -e ; \
	shopt -s nullglob ; \
	for file in $(BUILDDIR)/install.d/* ; do \
		echo "installing $${file}";\
	 	ACI=$(ACI) ROOT=$(ROOT) $(SHELL) $${file}; \
	done
	$(GOBIN)/actool build --overwrite $(ACI) $(GOBIN)/stage1.aci

modules: SHELL := /bin/bash
modules:
# Every module produces an install script into $(BUILDDIR)/install.d folder
	rm -rf $(BUILDDIR)/install.d/*
	rm -rf $(BUILDDIR)/aci/rootfs/*
	set -e ; \
	shopt -s nullglob ; \
	for subdir in $(SUBDIRS) ; do \
	    ( $(MAKE) -C $$subdir -f $$subdir.mk  ) || exit $$?; \
	done
