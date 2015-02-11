/**
 * Tool utility functions
 *
 * Copyright (c) 2014 Red Hat <pmoore@redhat.com>
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

#ifndef _UTIL_H
#define _UTIL_H

#include <inttypes.h>
#include <linux/audit.h>

#ifndef __AUDIT_ARCH_CONVENTION_MIPS64_N32
#define __AUDIT_ARCH_CONVENTION_MIPS64_N32	0x20000000
#endif

#ifndef AUDIT_ARCH_MIPS64N32
/* MIPS64N32 support was merged in 3.15 */
#define AUDIT_ARCH_MIPS64N32	(EM_MIPS|__AUDIT_ARCH_64BIT|\
				 __AUDIT_ARCH_CONVENTION_MIPS64_N32)
#endif

#ifndef AUDIT_ARCH_MIPSEL64N32
/* MIPSEL64N32 support was merged in 3.15 */
#define AUDIT_ARCH_MIPSEL64N32	(EM_MIPS|__AUDIT_ARCH_64BIT|__AUDIT_ARCH_LE|\
				 __AUDIT_ARCH_CONVENTION_MIPS64_N32)
#endif

#ifndef AUDIT_ARCH_AARCH64
/* AArch64 support for audit was merged in 3.17-rc1 */
#ifndef EM_AARCH64
#define EM_AARCH64 183
#endif
#ifndef EM_ARM
#define EM_ARM 40
#endif
#define AUDIT_ARCH_AARCH64	(EM_AARCH64|__AUDIT_ARCH_64BIT|__AUDIT_ARCH_LE)
#endif

extern uint32_t arch;

void exit_usage(const char *program);

uint16_t ttoh16(uint32_t arch, uint16_t val);
uint32_t ttoh32(uint32_t arch, uint32_t val);

uint32_t htot32(uint32_t arch, uint32_t val);
uint64_t htot64(uint32_t arch, uint64_t val);

#endif
