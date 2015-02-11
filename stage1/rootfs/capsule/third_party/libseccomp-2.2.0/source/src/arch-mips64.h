/**
 * Enhanced Seccomp MIPS64 Specific Code
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

#ifndef _ARCH_MIPS64_H
#define _ARCH_MIPS64_H

#include <inttypes.h>

#include "arch.h"
#include "system.h"

extern const struct arch_def arch_def_mips64;
extern const struct arch_def arch_def_mipsel64;

int mips64_syscall_resolve_name(const char *name);
const char *mips64_syscall_resolve_num(int num);

const char *mips64_syscall_iterate_name(unsigned int spot);

#endif