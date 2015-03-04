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

#ifndef _ARCH_X86_H
#define _ARCH_X86_H

#include <stdbool.h>

#include "arch.h"
#include "db.h"
#include "system.h"

extern const struct arch_def arch_def_x86;

int x86_syscall_resolve_name(const char *name);
const char *x86_syscall_resolve_num(int num);

const char *x86_syscall_iterate_name(unsigned int spot);

int x86_syscall_rewrite(const struct arch_def *arch, bool strict, int *syscall);

int x86_filter_rewrite(const struct arch_def *arch, bool strict,
		       int *syscall, struct db_api_arg *chain);

#endif
