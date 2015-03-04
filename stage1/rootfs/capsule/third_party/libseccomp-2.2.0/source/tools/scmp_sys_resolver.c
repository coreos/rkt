/**
 * Syscall resolver
 *
 * Copyright (c) 2012 Red Hat <pmoore@redhat.com>
 * Author: Paul Moore <pmoore@redhat.com>
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

/**
 * Print the usage information to stderr and exit
 * @param program the name of the current program being invoked
 *
 * Print the usage information and exit with EINVAL.
 *
 */
static void exit_usage(const char *program)
{
	fprintf(stderr,
		"usage: %s [-h] [-a <arch>] [-t] <name>|<number>\n",
		program);
	exit(EINVAL);
}

/**
 * main
 */
int main(int argc, char *argv[])
{
	int opt;
	int translate = 0;
	uint32_t arch;
	int sys_num;
	const char *sys_name;

	arch = seccomp_arch_native();

	/* parse the command line */
	while ((opt = getopt(argc, argv, "a:ht")) > 0) {
		switch (opt) {
		case 'a':
			arch = seccomp_arch_resolve_name(optarg);
			if (arch == 0)
				exit_usage(argv[0]);
			break;
		case 't':
			translate = 1;
			break;
		case 'h':
		default:
			/* usage information */
			exit_usage(argv[0]);
		}
	}

	/* sanity checks */
	if (optind >= argc)
		exit_usage(argv[0]);

	/* perform the syscall lookup */
	if (isdigit(argv[optind][0]) || argv[optind][0] == '-') {
		sys_num = atoi(argv[optind]);
		sys_name = seccomp_syscall_resolve_num_arch(arch, sys_num);
		printf("%s\n", (sys_name ? sys_name : "UNKNOWN"));
	} else if (translate) {
		sys_num = seccomp_syscall_resolve_name_rewrite(arch,
							       argv[optind]);
		printf("%d\n", sys_num);
	} else {
		sys_num = seccomp_syscall_resolve_name_arch(arch, argv[optind]);
		printf("%d\n", sys_num);
	}

	return 0;
}
