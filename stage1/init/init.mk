BGB_GO_FLAGS := $(strip -tags "$(KVM_HV_TAG)")
include stage1/makelib/aci_simple_go_bin.mk
