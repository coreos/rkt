# do not include systemd in stage1, let's find it at run-time

usr.done install: Makefile
	echo mkdir -p "\$${ROOT}" > install.tmp
	mv install.tmp install
	cp -f install ../aggregate/install.d/00usr
	touch usr.done

.PHONY: clean distclean

distclean: clean

clean:
	rm -Rf rootfs usr.done install

test:
	echo TODO
