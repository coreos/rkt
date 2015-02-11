/**
 * Enhanced Seccomp Architecture/Machine Specific Code
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

#ifndef _ARCH_H
#define _ARCH_H

#include <inttypes.h>
#include <stddef.h>
#include <stdbool.h>

#include <seccomp.h>

#include "system.h"

struct db_api_arg;

struct arch_def {
	uint32_t token;
	uint32_t token_bpf;
	enum {
		ARCH_SIZE_UNSPEC = 0,
		ARCH_SIZE_32 = 32,
		ARCH_SIZE_64 = 64,
	} size;
	enum {
		ARCH_ENDIAN_UNSPEC = 0,
		ARCH_ENDIAN_LITTLE,
		ARCH_ENDIAN_BIG,
	} endian;
};

/* arch_def for the current architecture */
extern const struct arch_def *arch_def_native;

/* NOTE: Syscall mappings can be found by running the following commands
 *	 on the specific architecture's include file:
 *	   # gcc -E -dM <file> | grep '__NR_'
 *	 where <file> in many cases is /usr/include/asm/unistd.h, however,
 *	 depending on the architecture you may need to use a different header.
 *	 Further, you can automatically format this list for use as a struct
 *	 initializer with the following command:
 *	   # gcc -E -dM <file> | grep '__NR_' | \
 *	     sed -e 's/#define[ \t]\+__NR_//' | sort | \
 *	     sed -e 's/\([^ \t]\+\)\([ \t]\+\)\([0-9]\+\)/\t{ \"\1\", \3 },/'
 *	 Finally, when creating a table/array of this structure, the final
 *	 sentinel entry should be "{ NULL, __NR_SCMP_ERROR }"; see the existing
 *	 tables as an example.
 */
struct arch_syscall_def {
	const char *name;
	unsigned int num;
};

#define DATUM_MAX	((scmp_datum_t)-1)
#define D64_LO(x)	((uint32_t)((uint64_t)(x) & 0x00000000ffffffff))
#define D64_HI(x)	((uint32_t)((uint64_t)(x) >> 32))

#define ARG_COUNT_MAX	6

int arch_valid(uint32_t arch);

const struct arch_def *arch_def_lookup(uint32_t token);
const struct arch_def *arch_def_lookup_name(const char *arch_name);

int arch_arg_count_max(const struct arch_def *arch);

int arch_arg_offset_lo(const struct arch_def *arch, unsigned int arg);
int arch_arg_offset_hi(const struct arch_def *arch, unsigned int arg);
int arch_arg_offset(const struct arch_def *arch, unsigned int arg);

int arch_syscall_resolve_name(const struct arch_def *arch, const char *name);
const char *arch_syscall_resolve_num(const struct arch_def *arch, int num);

int arch_syscall_translate(const struct arch_def *arch, int *syscall);
int arch_syscall_rewrite(const struct arch_def *arch, bool strict,
			 int *syscall);

int arch_filter_rewrite(const struct arch_def *arch,
			bool strict, int *syscall, struct db_api_arg *chain);

#endif
