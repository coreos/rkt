# custom kernel compilation
KERNEL_VERSION=4.1.1
KERNEL_FILE=linux-${KERNEL_VERSION}.tar.xz
KERNEL_DIR=linux-$(KERNEL_VERSION)
KERNEL_PATH=https://www.kernel.org/pub/linux/kernel/v4.x/

bzImage: $(KERNEL_DIR)
	./patches.sh $(KERNEL_DIR)
	cp cutdown-config $(KERNEL_DIR)/.config
	(cd $(KERNEL_DIR) ; make -j`nproc` bzImage)
	cp $(KERNEL_DIR)/arch/x86/boot/bzImage .

$(KERNEL_DIR)/Makefile: $(KERNEL_FILE)
	tar xf $(KERNEL_FILE) 

$(KERNEL_FILE):
	wget $(KERNEL_PATH)/$(KERNEL_FILE)

clean:
	rm -rf $(KERNEL_DIR)

