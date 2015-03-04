/**
 * Enhanced Seccomp x86 Specific Code
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

#include <stdlib.h>
#include <errno.h>
#include <linux/audit.h>

#include "arch.h"
#include "arch-x86.h"

/* x86 syscall numbers */
#define __x86_NR_socketcall		102
#define __x86_NR_ipc			117

const struct arch_def arch_def_x86 = {
	.token = SCMP_ARCH_X86,
	.token_bpf = AUDIT_ARCH_I386,
	.size = ARCH_SIZE_32,
	.endian = ARCH_ENDIAN_LITTLE,
};

/**
 * Rewrite a syscall value to match the architecture
 * @param arch the architecture definition
 * @param strict strict flag
 * @param syscall the syscall number
 *
 * Syscalls can vary across different architectures so this function rewrites
 * the syscall into the correct value for the specified architecture.  If
 * @strict is true then the function will fail if the syscall can not be
 * preservered, however, if @strict is false the function will do a "best
 * effort" rewrite and not fail. Returns zero on success, negative values on
 * failure.
 *
 */
int x86_syscall_rewrite(const struct arch_def *arch, bool strict, int *syscall)
{
	if ((*syscall) <= -100 && (*syscall) >= -117)
		*syscall = __x86_NR_socketcall;
	else if ((*syscall) <= -200 && (*syscall) >= -211)
		*syscall = __x86_NR_ipc;
	else if (((*syscall) < 0) && (strict))
		return -EDOM;

	return 0;
}

/**
 * Rewrite a filter rule to match the architecture specifics
 * @param arch the architecture definition
 * @param strict strict flag
 * @param syscall the syscall number
 * @param chain the argument filter chain
 *
 * Syscalls can vary across different architectures so this function handles
 * the necessary seccomp rule rewrites to ensure the right thing is done
 * regardless of the rule or architecture.  If @strict is true then the
 * function will fail if the entire filter can not be preservered, however,
 * if @strict is false the function will do a "best effort" rewrite and not
 * fail.  Returns zero on success, negative values on failure.
 *
 */
int x86_filter_rewrite(const struct arch_def *arch, bool strict,
		       int *syscall, struct db_api_arg *chain)
{
	unsigned int iter;
	int arg_max;

	arg_max = arch_arg_count_max(arch);
	if (arg_max < 0)
		return arg_max;

	if ((*syscall) <= -100 && (*syscall) >= -117) {
		for (iter = 0; iter < arg_max; iter++) {
			if ((chain[iter].valid != 0) && (strict))
				return -EINVAL;
		}
		chain[0].arg = 0;
		chain[0].op = SCMP_CMP_EQ;
		chain[0].mask = DATUM_MAX;
		chain[0].datum = abs(*syscall) % 100;
		chain[0].valid = 1;
		*syscall = __x86_NR_socketcall;
	} else if ((*syscall) <= -200 && (*syscall) >= -211) {
		for (iter = 0; iter < arg_max; iter++) {
			if ((chain[iter].valid != 0) && (strict))
				return -EINVAL;
		}
		chain[0].arg = 0;
		chain[0].op = SCMP_CMP_EQ;
		chain[0].mask = DATUM_MAX;
		chain[0].datum = abs(*syscall) % 200;
		chain[0].valid = 1;
		*syscall = __x86_NR_ipc;
	} else if (((*syscall) < 0) && (strict))
		return -EDOM;

	return 0;
}
