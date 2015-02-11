/**
 * Enhanced Seccomp MIPS Specific Code
 *
 * Copyright (c) 2014 Imagination Technologies Ltd.
 * Author: Markos Chandras <markos.chandras@imgtec.com>
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

#ifndef _ARCH_MIPS_H
#define _ARCH_MIPS_H

#include <inttypes.h>

#include "arch.h"
#include "system.h"

extern const struct arch_def arch_def_mips;
extern const struct arch_def arch_def_mipsel;

int mips_syscall_resolve_name(const char *name);
const char *mips_syscall_resolve_num(int num);

const char *mips_syscall_iterate_name(unsigned int spot);

#endif
