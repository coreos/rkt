/**
 * Enhanced Seccomp Architecture Sycall Checker
 *
 * Copyright (c) 2014 Red Hat <pmoore@redhat.com>
 * Author: Paul Moore <pmoore@redhat.com>
 *
 */

/*
 * This library is free software; you can redistribute it and/or modify it
 * under the terms of version 2.1 of the GNU Lesser General Public License as
 * published by the Free Software Foundation.
 *
 * This library is distributed in the hope that it will be useful, but WITHOUT
 * ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or
 * FITNESS FOR A PARTICULAR PURPOSE.  See the GNU Lesser General Public License
 * for more details.
 *
 * You should have received a copy of the GNU Lesser General Public License
 * along with this library; if not, see <http://www.gnu.org/licenses>.
 */

#include <errno.h>
#include <stdlib.h>
#include <stdio.h>
#include <ctype.h>
#include <string.h>
#include <unistd.h>

#include <seccomp.h>

#include "arch.h"
#include "arch-x86.h"
#include "arch-x86_64.h"
#include "arch-x32.h"
#include "arch-arm.h"
#include "arch-mips.h"
#include "arch-mips64.h"
#include "arch-mips64n32.h"
#include "arch-aarch64.h"

/**
 * Print the usage information to stderr and exit
 * @param program the name of the current program being invoked
 *
 * Print the usage information and exit with EINVAL.
 *
 */
static void exit_usage(const char *program)
{
	fprintf(stderr, "usage: %s [-h] [-a <arch>] [-o <offset>]\n", program);
	exit(EINVAL);
}

/**
 * main
 */
int main(int argc, char *argv[])
{
	int opt;
	const struct arch_def *arch = arch_def_native;
	int offset = 0;
	int iter;
	int sys_num;
	const char *sys_name;

	/* parse the command line */
	while ((opt = getopt(argc, argv, "a:o:h")) > 0) {
		switch (opt) {
		case 'a':
			arch = arch_def_lookup_name(optarg);
			if (arch == 0)
				exit_usage(argv[0]);
			break;
		case 'o':
			offset = atoi(optarg);
			break;
		case 'h':
		default:
			/* usage information */
			exit_usage(argv[0]);
		}
	}

	iter = 0;
	do {
		switch (arch->token) {
		case SCMP_ARCH_X86:
			sys_name = x86_syscall_iterate_name(iter);
			break;
		case SCMP_ARCH_X86_64:
			sys_name = x86_64_syscall_iterate_name(iter);
			break;
		case SCMP_ARCH_X32:
			sys_name = x32_syscall_iterate_name(iter);
			break;
		case SCMP_ARCH_ARM:
			sys_name = arm_syscall_iterate_name(iter);
			break;
		case SCMP_ARCH_MIPS:
		case SCMP_ARCH_MIPSEL:
			sys_name = mips_syscall_iterate_name(iter);
			break;
		case SCMP_ARCH_MIPS64:
		case SCMP_ARCH_MIPSEL64:
			sys_name = mips64_syscall_iterate_name(iter);
			break;
		case SCMP_ARCH_MIPS64N32:
		case SCMP_ARCH_MIPSEL64N32:
			sys_name = mips64n32_syscall_iterate_name(iter);
			break;
		case SCMP_ARCH_AARCH64:
			sys_name = aarch64_syscall_iterate_name(iter);
			break;
		default:
			/* invalid arch */
			exit_usage(argv[0]);
		}
		if (sys_name != NULL) {
			sys_num = arch_syscall_resolve_name(arch, sys_name);
			if (offset > 0 && sys_num > 0)
				sys_num -= offset;

			/* output the results */
			printf("%s\t%d\n", sys_name, sys_num);

			/* next */
			iter++;
		}
	} while (sys_name != NULL);

	return 0;
}
