BIN=capsule
SRC=args.c blacklist.c capabilities.c fds.c capsule.c mounts.c pty.c
HDRS=args.h blacklist.h capabilities.h fatal.h fds.h mounts.h pty.h
DEFS=defs/arguments.def defs/capabilities.def defs/namespaces.def defs/prepsteps.def 

# libseccomp is vendored because not all distributions make the .a available
SECCOMP=third_party/libseccomp-2.2.0
SECCOMPSRC=$(SECCOMP)/source
SECCOMPBUILD=$(SECCOMP)/build
SECCOMPLIB=$(SECCOMPBUILD)/src/.libs/libseccomp.a
SECCOMPINC=$(SECCOMPBUILD)/include
SECCOMPMAKE=$(SECCOMPBUILD)/Makefile

$(BIN): Makefile $(SRC) $(DEFS) $(HDRS) $(SECCOMPLIB)
	$(CC) $(CFLAGS) -D_GNU_SOURCE -I$(SECCOMPINC) -o $@ $(SRC) $(SECCOMPLIB) -static -s
	@cp install ../aggregate/install.d/10capsule

$(SECCOMPLIB): $(SECCOMPMAKE)
	make -C $(SECCOMPBUILD)

$(SECCOMPMAKE):
	cd $(SECCOMPSRC) && ./autogen.sh
	ln -sf ../../build/include/seccomp.h $(SECCOMPSRC)/include/seccomp.h	# work around minor seccomp build system fail
	mkdir $(SECCOMPBUILD) && cd $(SECCOMPBUILD) && ../source/configure

.PHONY: clean
clean:
	rm -Rf $(BIN) $(SECCOMPBUILD)

test:
	echo TODO
