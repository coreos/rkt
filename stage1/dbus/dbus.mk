include ../../makelib/*.mk

SRC := $(BUILDDIR)/src/$(RKT_STAGE1_DBUS_VER)
ISCRIPT := $(BUILDDIR)/install.d/02dbus.install
PWD := $(shell pwd)




.PHONY: install

install: $(BUILDDIR)/dbus.done
	@echo $(call dep-copy-fs,$(BUILDDIR)/dbus-installed) > $(ISCRIPT)
	@echo $(call dep-install-file,$(call find-so-deps, $(BUILDDIR)/dbus-installed)) >> $(ISCRIPT)
# we have built a special library that tepends on the desired module and copying the library linked to it
	@echo $(call dep-install-file,$(call find-so-deps, $(BUILDDIR)/nss_files/))  >> $(ISCRIPT)

	@echo $(call dep-systemd,dbus.socket,sockets.target.wants) >> $(ISCRIPT)
	@echo $(call dep-systemd,dbus.service,default.target.wants) >> $(ISCRIPT)

	@echo $(call dep-install-file,$(shell which ldconfig)) >> $(ISCRIPT)
	@echo $(call dep-install-file,$(shell which ldconfig.real)) >> $(ISCRIPT)



$(BUILDDIR)/dbus.done: $(BUILDDIR)/dbus.src.done $(BUILDDIR)/nss_files/test dbus.mk
	cd $(SRC) && ./autogen.sh
	cd $(SRC) && ./configure\
            --prefix=/usr \
            --sysconfdir=/etc \
            --localstatedir=/var \
            --libexecdir=$(USR_LIB_DIR)/dbus-1 \
            --libdir=$(USR_LIB_DIR) \
            --with-console-auth-dir=/run/console \
            --with-systemdsystemunitdir=$(USR_LIB_DIR)/systemd/system \
            --enable-systemd \
            --enable-verbose-mode \
            --disable-asserts \
            --enable-checks \
            --disable-xml-docs \
            --disable-doxygen-docs \
            --disable-ducktype-docs \
            --enable-abstract-sockets \
            --disable-selinux \
            --disable-apparmor \
            --disable-libaudit \
            --enable-inotify \
            --disable-console-owner-file \
            --disable-launchd \
            --disable-embedded-tests \
            --disable-modular-tests \
            --disable-tests \
            --disable-installed-tests \
            --enable-epoll \
            --disable-x11-autolaunch \
            --disable-Werror \
            --enable-ld-version-script \
            --disable-stats \
            --enable-user-session \
            --with-init-scripts=none \
            --with-system-socket=/run/dbus/system_bus_socket \
            --with-system-pid-file=/run/dbus/messagebus.pid

	$(MAKE) -C $(SRC) dbus
	$(MAKE) DESTDIR=$(BUILDDIR)/dbus-installed install-strip -C $(SRC)
	touch $(BUILDDIR)/dbus.done

$(BUILDDIR)/dbus.src.done:
	{ [ ! -e $(SRC) ] || rm -Rf $(SRC); }
	$(GIT) clone --branch $(RKT_STAGE1_DBUS_VER) --depth 1 $(RKT_STAGE1_DBUS_SRC) $(SRC)
	touch $(BUILDDIR)/dbus.src.done


# NOTE: glibc needs libnss_files.so to extract users from /etc/password and /etc/group:
# we are compiling a little program that is linked to the required module to extract dependencies
# this allows us to run dbus not as root
$(BUILDDIR)/nss_files/test: $(PWD)/nss_files/test.c
	mkdir -p $(BUILDDIR)/nss_files
	cd $(PWD)/nss_files && $(CC) $(CFLAGS) -o $@ test.c $(NSS_FILES_LIB)
