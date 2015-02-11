/**
 * Seccomp BPF Translator
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

#ifndef _TRANSLATOR_BPF_H
#define _TRANSLATOR_BPF_H

#include <inttypes.h>

#include "arch.h"
#include "db.h"
#include "system.h"

/* NOTE - do not change this structure, it is part of the prctl() API */
struct bpf_program {
	uint16_t blk_cnt;
	bpf_instr_raw *blks;
};
#define BPF_PGM_SIZE(x) \
	((x)->blk_cnt * sizeof(*((x)->blks)))

struct bpf_program *gen_bpf_generate(const struct db_filter_col *col);
void gen_bpf_release(struct bpf_program *program);

#endif
