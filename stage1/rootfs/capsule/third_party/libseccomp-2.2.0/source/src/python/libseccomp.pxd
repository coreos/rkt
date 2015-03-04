#
# Seccomp Library Python Bindings
#
# Copyright (c) 2012,2013 Red Hat <pmoore@redhat.com>
# Author: Paul Moore <pmoore@redhat.com>
#

#
# This library is free software; you can redistribute it and/or modify it
# under the terms of version 2.1 of the GNU Lesser General Public License as
# published by the Free Software Foundation.
#
# This library is distributed in the hope that it will be useful, but WITHOUT
# ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or
# FITNESS FOR A PARTICULAR PURPOSE.  See the GNU Lesser General Public License
# for more details.
#
# You should have received a copy of the GNU Lesser General Public License
# along with this library; if not, see <http://www.gnu.org/licenses>.
#

from libc.stdint cimport uint8_t, uint32_t, uint64_t

cdef extern from "seccomp.h":

    ctypedef void* scmp_filter_ctx

    cdef enum:
        SCMP_ARCH_NATIVE
        SCMP_ARCH_X86
        SCMP_ARCH_X86_64
        SCMP_ARCH_X32
        SCMP_ARCH_ARM
        SCMP_ARCH_AARCH64
        SCMP_ARCH_MIPS
        SCMP_ARCH_MIPS64
        SCMP_ARCH_MIPS64N32
        SCMP_ARCH_MIPSEL
        SCMP_ARCH_MIPSEL64
        SCMP_ARCH_MIPSEL64N32

    cdef enum scmp_filter_attr:
        SCMP_FLTATR_ACT_DEFAULT
        SCMP_FLTATR_ACT_BADARCH
        SCMP_FLTATR_CTL_NNP

    cdef enum scmp_compare:
        SCMP_CMP_NE
        SCMP_CMP_LT
        SCMP_CMP_LE
        SCMP_CMP_EQ
        SCMP_CMP_GE
        SCMP_CMP_GT
        SCMP_CMP_MASKED_EQ

    cdef enum:
        SCMP_ACT_KILL
        SCMP_ACT_TRAP
        SCMP_ACT_ALLOW
    unsigned int SCMP_ACT_ERRNO(int errno)
    unsigned int SCMP_ACT_TRACE(int value)

    ctypedef uint64_t scmp_datum_t

    cdef struct scmp_arg_cmp:
        unsigned int arg
        scmp_compare op
        scmp_datum_t datum_a
        scmp_datum_t datum_b

    scmp_filter_ctx seccomp_init(uint32_t def_action)
    int seccomp_reset(scmp_filter_ctx ctx, uint32_t def_action)
    void seccomp_release(scmp_filter_ctx ctx)

    int seccomp_merge(scmp_filter_ctx ctx_dst, scmp_filter_ctx ctx_src)

    uint32_t seccomp_arch_resolve_name(char *arch_name)
    uint32_t seccomp_arch_native()
    int seccomp_arch_exist(scmp_filter_ctx ctx, int arch_token)
    int seccomp_arch_add(scmp_filter_ctx ctx, int arch_token)
    int seccomp_arch_remove(scmp_filter_ctx ctx, int arch_token)

    int seccomp_load(scmp_filter_ctx ctx)

    int seccomp_attr_get(scmp_filter_ctx ctx,
                         scmp_filter_attr attr, uint32_t* value)
    int seccomp_attr_set(scmp_filter_ctx ctx,
                         scmp_filter_attr attr, uint32_t value)

    char *seccomp_syscall_resolve_num_arch(int arch_token, int num)
    int seccomp_syscall_resolve_name_arch(int arch_token, char *name)
    int seccomp_syscall_resolve_name_rewrite(int arch_token, char *name)
    int seccomp_syscall_resolve_name(char *name)
    int seccomp_syscall_priority(scmp_filter_ctx ctx,
                                 int syscall, uint8_t priority)

    int seccomp_rule_add(scmp_filter_ctx ctx, uint32_t action,
                         int syscall, unsigned int arg_cnt, ...)

    int seccomp_rule_add_exact(scmp_filter_ctx ctx, uint32_t action,
                               int syscall, unsigned int arg_cnt, ...)

    int seccomp_export_pfc(scmp_filter_ctx ctx, int fd)
    int seccomp_export_bpf(scmp_filter_ctx ctx, int fd)

# kate: syntax python;
# kate: indent-mode python; space-indent on; indent-width 4; mixedindent off;
