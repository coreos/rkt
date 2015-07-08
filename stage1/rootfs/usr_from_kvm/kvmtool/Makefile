lkvm: kvmtool
	@make -C kvmtool lkvm-static
	@mv kvmtool/lkvm-static lkvm

kvmtool:
	@git clone https://kernel.googlesource.com/pub/scm/linux/kernel/git/will/kvmtool

.PHONY: clean
clean:
	rm lkvm
	rm -rf kvmtool
