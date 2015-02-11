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

#include <linux/audit.h>

#include "arch.h"
#include "arch-mips64.h"

const struct arch_def arch_def_mips64 = {
	.token = SCMP_ARCH_MIPS64,
	.token_bpf = AUDIT_ARCH_MIPS64,
	.size = ARCH_SIZE_64,
	.endian = ARCH_ENDIAN_BIG,
};

const struct arch_def arch_def_mipsel64 = {
	.token = SCMP_ARCH_MIPSEL64,
	.token_bpf = AUDIT_ARCH_MIPSEL64,
	.size = ARCH_SIZE_64,
	.endian = ARCH_ENDIAN_LITTLE,
};
