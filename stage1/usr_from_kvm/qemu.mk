$(call setup-stamp-file,QEMU_STAMP)
QEMU_TMPDIR := $(UFK_TMPDIR)/qemu
QEMU_SRCDIR := $(QEMU_TMPDIR)/src
QEMU_BINARY := $(QEMU_SRCDIR)/x86_64-softmmu/qemu-system-x86_64
QEMU_BIOS_BINARIES := bios-256k.bin \
    kvmvapic.bin \
    linuxboot.bin \
    vgabios-stdvga.bin \
    efi-virtio.rom

QEMU_CONFIGURATION_OPTS := --disable-bsd-user --disable-docs --disable-guest-agent --disable-guest-agent-msi \
    --disable-sdl --disable-gtk --disable-vte --disable-curses --disable-cocoa --disable-brlapi --disable-vnc \
    --disable-seccomp --disable-curl --disable-bluez --disable-cap-ng --disable-rbd --disable-libiscsi \
    --disable-libnfs --disable-smartcard --disable-libusb --disable-glusterfs --disable-archipelago --disable-vhdx \
    --disable-tcmalloc --disable-jemalloc --disable-debug-info --enable-virtfs --static --target-list=x86_64-softmmu
QEMU_ACI_BINARY := $(S1_RF_ACIROOTFSDIR)/qemu
QEMU_GIT := git://git.qemu-project.org/qemu.git
# Latest stable version of QEMU - 2.5.1
QEMU_GIT_COMMIT := 	v2.5.1


$(call setup-stamp-file,QEMU_BUILD_STAMP,/build)
$(call setup-stamp-file,QEMU_BIOS_BUILD_STAMP,/bios_build)
$(call setup-stamp-file,QEMU_CONF_STAMP,/conf)
$(call setup-stamp-file,QEMU_CLONE_STAMP,/clone)

S1_RF_SECONDARY_STAMPS += $(QEMU_STAMP)
S1_RF_INSTALL_FILES += $(QEMU_BINARY):$(QEMU_ACI_BINARY):-
INSTALL_DIRS += \
    $(QEMU_SRCDIR) :- \
    $(QEMU_TMPDIR) :-

$(call generate-stamp-rule,$(QEMU_STAMP),$(QEMU_CLONE_STAMP) $(QEMU_CONF_STAMP) $(QEMU_BUILD_STAMP) $(QEMU_ACI_BINARY) $(QEMU_BIOS_BUILD_STAMP),,)

$(QEMU_BINARY): $(QEMU_BUILD_STAMP)

$(call generate-stamp-rule,$(QEMU_BIOS_BUILD_STAMP),$(QEMU_CONF_STAMP),, \
  	for bios in $(QEMU_BIOS_BINARIES); do \
        $(call vb,vt,COPY BIOS,$$$${bios}) \
  	  	cp $(QEMU_SRCDIR)/pc-bios/$$$${bios} $(S1_RF_ACIROOTFSDIR)/$$$${bios} $(call vl2,>/dev/null); \
    done)


$(call generate-stamp-rule,$(QEMU_BUILD_STAMP),$(QEMU_CONF_STAMP),, \
    $(call vb,vt,BUILD EXT,qemu) \
	$$(MAKE) $(call vl2,--silent) -C "$(QEMU_SRCDIR)" $(call vl2,>/dev/null))

$(call generate-stamp-rule,$(QEMU_CONF_STAMP),$(QEMU_CLONE_STAMP),, \
	$(call vb,vt,CONFIG EXT,qemu) \
	cd $(QEMU_SRCDIR); ./configure $(QEMU_CONFIGURATOR) $(QEMU_CONFIGURATION_OPTS) $(call vl2,>/dev/null))

GCL_REPOSITORY := $(QEMU_GIT)
GCL_DIRECTORY := $(QEMU_SRCDIR)
GCL_COMMITTISH := $(QEMU_GIT_COMMIT)
GCL_EXPECTED_FILE := Makefile
GCL_TARGET := $(QEMU_CLONE_STAMP)
GCL_DO_CHECK :=

include makelib/git.mk

$(call undefine-namespaces,QEMU)
