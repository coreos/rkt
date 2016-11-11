ASGB_FLAVORS := $(filter kvm,$(STAGE1_FLAVORS))

include stage1/makelib/aci_simple_go_bin.mk
