# build systemd from source, produce an install hook selecting the needed files and host dependencies

# make needs to use bash for nullglob
SHELL := /bin/bash

usr.done: Makefile install
	cp -f install ../aggregate/install.d/00usr
	touch usr.done

# prepare the install file
install: Makefile host_deps.txt
	echo mkdir -p "\$${ROOT}" > install.tmp
	echo cp -af ../usr_from_src/systemd_build/installed/. \"\$${ROOT}\" >> install.tmp
	cat host_deps.txt | while read dep; do echo install -D \"$${dep}\" \"\$${ROOT}/$${dep}\"; done >> install.tmp
	echo ln -sf src \"\$${ROOT}/flavor\" >> install.tmp
	mv install.tmp install

# discover host library dependencies for all the ELF executables in systemd_build/installed, note the LD_LIBRARY_PATH= to find systemd-produced libraries.
host_deps.txt: Makefile systemd.done bash.done
	cd systemd_build/installed && find -type f | xargs file  | grep ELF | cut -f1 -d: | LD_LIBRARY_PATH=/usr/lib64 xargs ldd | grep -v ^\\. | grep '/' | sed  -e 's/^[[:space:]]*//' -e 's/.*=> //' -e 's/ (0x[0-9a-f]*)$$//' | grep -v ^[^/] | sort -u > ../../host_deps.txt.tmp
	mv host_deps.txt.tmp host_deps.txt

# grab bash from the host
bash.done: Makefile systemd.done
	cp -af `which bash` systemd_build/installed/usr/bin/bash
	touch bash.done

# configure, build, and install systemd
systemd.done: Makefile
	{ [ ! -e systemd_build ] || rm -Rf systemd_build; }
	mkdir -p systemd_build/installed
	rpm -ql clr-systemd-config-config clr-systemd-config-data systemd glibc-utils nss-altfiles-lib systemd-libs linux-container-lkvm linux-container util-linux-bin | tar -cf - -T - | tar -xf - -C systemd_build/installed
	touch systemd.done

.PHONY: clean distclean
clean:
	rm -Rf systemd_build systemd.done bash.done host_deps.txt rootfs usr.done install

distclean: clean
	rm -Rf systemd

test:
	echo TODO
