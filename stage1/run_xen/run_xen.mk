ETK_FLAVORS := $(filter xen,$(STAGE1_FLAVORS))

ifneq ($(ETK_FLAVORS),)

ASGB_FLAVORS := $(ETK_FLAVORS)
MK_FILENAME := run
include stage1/makelib/aci_simple_go_bin.mk

endif

$(undefine-namespaces,ETK)
