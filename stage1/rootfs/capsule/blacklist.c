// Copyright 2015 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

#include <errno.h>
#include <linux/netlink.h>
#include <seccomp.h>
#include <stdio.h>
#include <string.h>
#include <sys/socket.h>
#include <sys/syscall.h>
#include <sys/types.h>
#include <unistd.h>

#include "args.h"
#include "fatal.h"

/* apply syscall blacklist (for now just the default, in future args should inform it) */
void blacklist_apply(args_t *args)
{
	const int	default_bl[] = {
#define	sys_bl(_sym, _desc)	SCMP_SYS(_sym),
#include "defs/blacklist.def"
	};
	const int	*bl = default_bl;
	int		bl_len = sizeof(default_bl) / sizeof(default_bl[0]);
	int		err, i;
	scmp_filter_ctx	sc;

	sc = seccomp_init(SCMP_ACT_ALLOW);

	/* add all architectures, NATIVE is already present */
#define add_arch(_ctxt, _arch)					\
	exit_if((err = seccomp_arch_add(_ctxt, _arch)) < 0 &&	\
		 err != -EEXIST,				\
		"failed to add architecture " #_arch);

	add_arch(sc, SCMP_ARCH_X86);
	add_arch(sc, SCMP_ARCH_X86_64);
	add_arch(sc, SCMP_ARCH_X32);

	for(i = 0; i < bl_len; i++) {
		exit_if(seccomp_rule_add(sc, SCMP_ACT_ERRNO(EPERM),
					 bl[i], 0) < 0,
			"unable to add syscall filter");
	}

	/* disable creation of audit sockets just like nspawn does */
	exit_if(seccomp_rule_add(sc, SCMP_ACT_ERRNO(EAFNOSUPPORT),
				 SCMP_SYS(socket), 2,
				 SCMP_A0(SCMP_CMP_EQ, AF_NETLINK),
				 SCMP_A2(SCMP_CMP_EQ, NETLINK_AUDIT)) < 0,
		"unable to add audit socket filter");
	exit_if(seccomp_attr_set(sc, SCMP_FLTATR_CTL_NNP, 0) < 0,
		"unable to set NO_NEW_PRIVS to off");

	/* we will fail here if the kernel doesn't have CONFIG_SECCOMP */
	exit_if((err = seccomp_load(sc)) < 0 && err != -EINVAL,
		"error loading SECCOMP filter: %s", strerror(-err));

	/* TODO(vc): only ignore the error if requested on the cli? */
	if(err < 0) {
		fprintf(stderr, "Ignoring seccomp_load() failure, "
				"enable CONFIG_SECCOMP in kernel.\n");
	}
}

