# aci directory should be the last one
_S1_SS_SUBDIRS_ := \
	enter \
	enter_kvm \
	enterexec \
	diagnostic \
	gc \
	init \
	net \
	net-plugins \
	prepare-app \
	reaper \
	stop \
	stop_kvm \
	app-add \
	app-rm \
	app-start \
	app-stop \
	units \
	aci

$(call inc-many,$(foreach f,$(_S1_SS_SUBDIRS_),$f/$f.mk))

$(call undefine-namespaces,S1_SS _S1_SS)
