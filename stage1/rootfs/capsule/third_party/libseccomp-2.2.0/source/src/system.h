/**
 * Seccomp System Interfaces
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

#ifndef _SYSTEM_H
#define _SYSTEM_H

#include <linux/filter.h>
#include <linux/prctl.h>

#include "configure.h"

struct db_filter_col;

#ifdef HAVE_LINUX_SECCOMP_H

/* system header file */
#include <linux/seccomp.h>

#else

/* NOTE: the definitions below were taken from the Linux Kernel sources */
#include <linux/types.h>

/* Valid values for seccomp.mode and prctl(PR_SET_SECCOMP, <mode>) */
#define SECCOMP_MODE_DISABLED	0 /* seccomp is not in use. */
#define SECCOMP_MODE_STRICT	1 /* uses hard-coded filter. */
#define SECCOMP_MODE_FILTER	2 /* uses user-supplied filter. */

/*
 * All BPF programs must return a 32-bit value.
 * The bottom 16-bits are for optional return data.
 * The upper 16-bits are ordered from least permissive values to most.
 *
 * The ordering ensures that a min_t() over composed return values always
 * selects the least permissive choice.
 */
#define SECCOMP_RET_KILL	0x00000000U /* kill the task immediately */
#define SECCOMP_RET_TRAP	0x00030000U /* disallow and force a SIGSYS */
#define SECCOMP_RET_ERRNO	0x00050000U /* returns an errno */
#define SECCOMP_RET_TRACE	0x7ff00000U /* pass to a tracer or disallow */
#define SECCOMP_RET_ALLOW	0x7fff0000U /* allow */

/* Masks for the return value sections. */
#define SECCOMP_RET_ACTION	0x7fff0000U
#define SECCOMP_RET_DATA	0x0000ffffU

/*
 * struct seccomp_data - the format the BPF program executes over.
 * @nr: the system call number
 * @arch: indicates system call convention as an AUDIT_ARCH_* value
 *	  as defined in <linux/audit.h>.
 * @instruction_pointer: at the time of the system call.
 * @args: up to 6 system call arguments always stored as 64-bit values
 *	  regardless of the architecture.
 */
struct seccomp_data {
	int nr;
	__u32 arch;
	__u64 instruction_pointer;
	__u64 args[6];
};

#endif /* HAVE_LINUX_SECCOMP_H */

/* rename some of the socket filter types to make more sense */
typedef struct sock_filter bpf_instr_raw;

/* no new privs defintions */
#ifndef PR_SET_NO_NEW_PRIVS
#define PR_SET_NO_NEW_PRIVS		38
#endif

#ifndef PR_GET_NO_NEW_PRIVS
#define PR_GET_NO_NEW_PRIVS		39
#endif

/* operations for the seccomp() syscall */
#ifndef SECCOMP_MODE_STRICT
#define SECCOMP_SET_MODE_STRICT		0
#endif
#ifndef SECCOMP_SET_MODE_FILTER
#define SECCOMP_SET_MODE_FILTER		1
#endif

/* flags for the seccomp() syscall */
#ifndef SECCOMP_FILTER_FLAG_TSYNC
#define SECCOMP_FILTER_FLAG_TSYNC	1
#endif

int sys_chk_seccomp_flag(int flag);

int sys_filter_load(const struct db_filter_col *col);

#endif
