# derives a base root fs from a CoreOS with addtional hypervisor components (kernel/kvm tool)
#

usr.done: Makefile install kernel/bzImage kvmtool/lkvm
	$(MAKE) -C ../usr_from_coreos
	cp -f install ../aggregate/install.d/00usr

kvmtool/lkvm: 
	$(MAKE) -C kvmtool

kernel/bzImage:
	$(MAKE) -C kernel

.PHONY: clean
clean:
	$(MAKE) -C usr_from_coreos clean
	rm usr.done
