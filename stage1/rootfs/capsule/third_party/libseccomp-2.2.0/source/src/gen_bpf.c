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

#include <errno.h>
#include <inttypes.h>
#include <stdlib.h>
#include <stdio.h>
#include <string.h>
#include <stdbool.h>

#ifndef _BSD_SOURCE
#define _BSD_SOURCE
#endif
#include <endian.h>

#include <seccomp.h>

#include "arch.h"
#include "arch-x32.h"
#include "gen_bpf.h"
#include "db.h"
#include "hash.h"
#include "system.h"

/* allocation increments */
#define AINC_BLK			2
#define AINC_PROG			64

struct acc_state {
	int32_t offset;
	uint32_t mask;
};

enum bpf_jump_type {
	TGT_NONE = 0,
	TGT_K,				/* immediate "k" value */
	TGT_NXT,			/* fall through to the next block */
	TGT_IMM,			/* resolved immediate value */
	TGT_PTR_DB,			/* pointer to part of the filter db */
	TGT_PTR_BLK,			/* pointer to an instruction block */
	TGT_PTR_HSH,			/* pointer to a block hash table */
};

struct bpf_jump {
	union {
		uint8_t imm_j;
		uint32_t imm_k;
		uint64_t hash;
		struct db_arg_chain_tree *db;
		struct bpf_blk *blk;
		unsigned int nxt;
	} tgt;
	enum bpf_jump_type type;
};
#define _BPF_OP(a,x) \
	(_htot16(a,x))
#define _BPF_JMP_NO \
	((struct bpf_jump) { .type = TGT_NONE })
#define _BPF_JMP_NXT(x) \
	((struct bpf_jump) { .type = TGT_NXT, .tgt = { .nxt = (x) } })
#define _BPF_JMP_IMM(x) \
	((struct bpf_jump) { .type = TGT_IMM, .tgt = { .imm_j = (x) } })
#define _BPF_JMP_DB(x) \
	((struct bpf_jump) { .type = TGT_PTR_DB, .tgt = { .db = (x) } })
#define _BPF_JMP_BLK(x) \
	((struct bpf_jump) { .type = TGT_PTR_BLK, .tgt = { .blk = (x) } })
#define _BPF_JMP_HSH(x) \
	((struct bpf_jump) { .type = TGT_PTR_HSH, .tgt = { .hash = (x) } })
#define _BPF_K(a,x) \
	((struct bpf_jump) { .type = TGT_K, .tgt = { .imm_k = _htot32(a,x) } })
#define _BPF_JMP_MAX			255
#define _BPF_JMP_MAX_RET		255

struct bpf_instr {
	uint16_t op;
	struct bpf_jump jt;
	struct bpf_jump jf;
	struct bpf_jump k;
};
#define _BPF_OFFSET_SYSCALL		(offsetof(struct seccomp_data, nr))
#define _BPF_SYSCALL(a)			_BPF_K(a,_BPF_OFFSET_SYSCALL)

struct bpf_blk {
	struct bpf_instr *blks;
	unsigned int blk_cnt;
	unsigned int blk_alloc;

	/* priority - higher is better */
	unsigned int priority;

	/* status flags */
	bool flag_hash;			/* added to the hash table */
	bool flag_dup;			/* duplicate block and in use */
	bool flag_unique;		/* ->blks is unique to this block */

	/* original db_arg_chain_tree node */
	const struct db_arg_chain_tree *node;

	/* used during block assembly */
	uint64_t hash;
	struct bpf_blk *hash_nxt;
	struct bpf_blk *prev, *next;
	struct bpf_blk *lvl_prv, *lvl_nxt;
	struct acc_state acc_state;
};
#define _BLK_MSZE(x) \
	((x)->blk_cnt * sizeof(*((x)->blks)))

struct bpf_hash_bkt {
	struct bpf_blk *blk;
	struct bpf_hash_bkt *next;
	unsigned int found;
};

#define _BPF_HASH_BITS			8
#define _BPF_HASH_SIZE			(1 << _BPF_HASH_BITS)
#define _BPF_HASH_MASK			(_BPF_HASH_BITS - 1)
struct bpf_state {
	/* block hash table */
	struct bpf_hash_bkt *htbl[_BPF_HASH_SIZE];

	/* filter attributes */
	const struct db_filter_attr *attr;
	/* default action */
	uint64_t def_hsh;

	/* target arch - NOTE: be careful, temporary use only! */
	const struct arch_def *arch;

	/* bpf program */
	struct bpf_program *bpf;
};

/**
 * Populate a BPF instruction
 * @param _ins the BPF instruction
 * @param _op the BPF operand
 * @param _jt the BPF jt value
 * @param _jf the BPF jf value
 * @param _k the BPF k value
 *
 * Set the given values on the provided bpf_instr struct.
 *
 */
#define _BPF_INSTR(_ins,_op,_jt,_jf,_k) \
	do { \
		memset(&(_ins), 0, sizeof(_ins)); \
		(_ins).op = (_op); \
		(_ins).jt = _jt; \
		(_ins).jf = _jf; \
		(_ins).k = _k; \
	} while (0)

static struct bpf_blk *_gen_bpf_chain(struct bpf_state *state,
				      const struct db_sys_list *sys,
				      const struct db_arg_chain_tree *chain,
				      const struct bpf_jump *nxt_jump,
				      struct acc_state *a_state);

static struct bpf_blk *_hsh_remove(struct bpf_state *state, uint64_t h_val);
static struct bpf_blk *_hsh_find(const struct bpf_state *state, uint64_t h_val);

/**
 * Convert a 16-bit host integer into the target's endianess
 * @param arch the architecture definition
 * @param val the 16-bit integer
 *
 * Convert the endianess of the supplied value and return it to the caller.
 *
 */
uint16_t _htot16(const struct arch_def *arch, uint16_t val)
{
	if (arch->endian == ARCH_ENDIAN_LITTLE)
		return htole16(val);
	else
		return htobe16(val);
}

/**
 * Convert a 32-bit host integer into the target's endianess
 * @param arch the architecture definition
 * @param val the 32-bit integer
 *
 * Convert the endianess of the supplied value and return it to the caller.
 *
 */
uint32_t _htot32(const struct arch_def *arch, uint32_t val)
{
	if (arch->endian == ARCH_ENDIAN_LITTLE)
		return htole32(val);
	else
		return htobe32(val);
}

/**
 * Free the BPF instruction block
 * @param state the BPF state
 * @param blk the BPF instruction block
 *
 * Free the BPF instruction block, any linked blocks are preserved and the hash
 * table is not modified.  In general, you probably want to use _blk_free()
 * instead.
 *
 */
static void __blk_free(struct bpf_state *state, struct bpf_blk *blk)
{
	struct bpf_blk *b_tmp;

	while (blk->hash_nxt != NULL) {
		b_tmp = blk->hash_nxt;
		blk->hash_nxt = b_tmp->hash_nxt;
		if (!b_tmp->flag_dup)
			free(b_tmp);
	}
	if (blk->blks != NULL && blk->flag_unique)
		free(blk->blks);
	free(blk);
}

/**
* Free the BPF instruction block
 * @param state the BPF state
 * @param blk the BPF instruction block
 *
 * Free the BPF instruction block including any linked blocks.  The hash table
 * is updated to reflect the newly removed block(s).
 *
 */
static void _blk_free(struct bpf_state *state, struct bpf_blk *blk)
{
	int iter;
	struct bpf_blk *b_iter;
	struct bpf_instr *i_iter;

	if (blk == NULL)
		return;

	/* remove this block from the hash table */
	_hsh_remove(state, blk->hash);

	/* run through the block freeing TGT_PTR_{BLK,HSH} jump targets */
	for (iter = 0; iter < blk->blk_cnt; iter++) {
		i_iter = &blk->blks[iter];
		switch (i_iter->jt.type) {
		case TGT_PTR_BLK:
			_blk_free(state, i_iter->jt.tgt.blk);
			break;
		case TGT_PTR_HSH:
			b_iter = _hsh_find(state, i_iter->jt.tgt.hash);
			_blk_free(state, b_iter);
			break;
		default:
			/* do nothing */
			break;
		}
		switch (i_iter->jf.type) {
		case TGT_PTR_BLK:
			_blk_free(state, i_iter->jf.tgt.blk);
			break;
		case TGT_PTR_HSH:
			b_iter = _hsh_find(state, i_iter->jf.tgt.hash);
			_blk_free(state, b_iter);
			break;
		default:
			/* do nothing */
			break;
		}
	}
	__blk_free(state, blk);
}

/**
 * Append a new BPF instruction to an instruction block
 * @param state the BPF state
 * @param blk the existing instruction block, or NULL
 * @param instr the new instruction
 *
 * Add the new BPF instruction to the end of the give instruction block.  If
 * the given instruction block is NULL, a new block will be allocated.  Returns
 * a pointer to the block on success, NULL on failure, and in the case of
 * failure the instruction block is free'd.
 *
 */
static struct bpf_blk *_blk_append(struct bpf_state *state,
				   struct bpf_blk *blk,
				   const struct bpf_instr *instr)
{
	struct bpf_instr *new;

	if (blk == NULL) {
		blk = malloc(sizeof(*blk));
		if (blk == NULL)
			return NULL;
		memset(blk, 0, sizeof(*blk));
		blk->flag_unique = true;
	}
	if ((blk->blk_cnt + 1) > blk->blk_alloc) {
		blk->blk_alloc += AINC_BLK;
		new = realloc(blk->blks, blk->blk_alloc * sizeof(*(blk->blks)));
		if (new == NULL) {
			_blk_free(state, blk);
			return NULL;
		}
		blk->blks = new;
	}
	memcpy(&blk->blks[blk->blk_cnt++], instr, sizeof(*instr));

	return blk;
}

/**
 * Append a block of BPF instructions to the final BPF program
 * @param prg the BPF program
 * @param blk the BPF instruction block
 *
 * Add the BPF instruction block to the end of the BPF program and perform the
 * necssary translation.  Returns zero on success, negative values on failure
 * and in the case of failure the BPF program is free'd.
 *
 */
static int _bpf_append_blk(struct bpf_program *prg, const struct bpf_blk *blk)
{
	int rc;
	bpf_instr_raw *i_new;
	bpf_instr_raw *i_iter;
	unsigned int old_cnt = prg->blk_cnt;
	unsigned int iter;

	/* (re)allocate the program memory */
	prg->blk_cnt += blk->blk_cnt;
	i_new = realloc(prg->blks, BPF_PGM_SIZE(prg));
	if (i_new == NULL) {
		rc = -ENOMEM;
		goto bpf_append_blk_failure;
	}
	prg->blks = i_new;

	/* transfer and translate the blocks to raw instructions */
	for (iter = 0; iter < blk->blk_cnt; iter++) {
		i_iter = &(prg->blks[old_cnt + iter]);

		i_iter->code = blk->blks[iter].op;
		switch (blk->blks[iter].jt.type) {
		case TGT_NONE:
			i_iter->jt = 0;
			break;
		case TGT_IMM:
			/* jump to the value specified */
			i_iter->jt = blk->blks[iter].jt.tgt.imm_j;
			break;
		default:
			/* fatal error - we should never get here */
			rc = -EFAULT;
			goto bpf_append_blk_failure;
		}
		switch (blk->blks[iter].jf.type) {
		case TGT_NONE:
			i_iter->jf = 0;
			break;
		case TGT_IMM:
			/* jump to the value specified */
			i_iter->jf = blk->blks[iter].jf.tgt.imm_j;
			break;
		default:
			/* fatal error - we should never get here */
			rc = -EFAULT;
			goto bpf_append_blk_failure;
		}
		switch (blk->blks[iter].k.type) {
		case TGT_NONE:
			i_iter->k = 0;
			break;
		case TGT_K:
			i_iter->k = blk->blks[iter].k.tgt.imm_k;
			break;
		default:
			/* fatal error - we should never get here */
			rc = -EFAULT;
			goto bpf_append_blk_failure;
		}
	}

	return prg->blk_cnt;

bpf_append_blk_failure:
	prg->blk_cnt = 0;
	free(prg->blks);
	return rc;
}

/**
 * Free the BPF program
 * @param prg the BPF program
 *
 * Free the BPF program.  None of the associated BPF state used to generate the
 * BPF program is released in this function.
 *
 */
static void _program_free(struct bpf_program *prg)
{
	if (prg == NULL)
		return;

	if (prg->blks != NULL)
		free(prg->blks);
	free(prg);
}

/**
 * Free the BPF state
 * @param the BPF state
 *
 * Free all of the BPF state, including the BPF program if present.
 *
 */
static void _state_release(struct bpf_state *state)
{
	unsigned int bkt;
	struct bpf_hash_bkt *iter;

	if (state == NULL)
		return;

	/* release all of the hash table entries */
	for (bkt = 0; bkt < _BPF_HASH_SIZE; bkt++) {
		while (state->htbl[bkt]) {
			iter = state->htbl[bkt];
			state->htbl[bkt] = iter->next;
			__blk_free(state, iter->blk);
			free(iter);
		}
	}
	_program_free(state->bpf);

	memset(state, 0, sizeof(*state));
}

/**
 * Add an instruction block to the BPF state hash table
 * @param state the BPF state
 * @param blk_p pointer to the BPF instruction block
 * @param found initial found value (see _hsh_find_once() for description)
 *
 * This function adds an instruction block to the hash table, and frees the
 * block if an identical instruction block already exists, returning a pointer
 * to the original block in place of the given block.  Returns zero on success
 * and negative values on failure.
 *
 */
static int _hsh_add(struct bpf_state *state, struct bpf_blk **blk_p,
		    unsigned int found)
{
	uint64_t h_val;
	struct bpf_hash_bkt *h_new, *h_iter, *h_prev = NULL;
	struct bpf_blk *blk = *blk_p;
	struct bpf_blk *b_iter;

	if (blk->flag_hash)
		return 0;

	h_new = malloc(sizeof(*h_new));
	if (h_new == NULL)
		return -ENOMEM;
	memset(h_new, 0, sizeof(*h_new));

	/* generate the hash */
	h_val = jhash(blk->blks, _BLK_MSZE(blk), 0);
	blk->hash = h_val;
	blk->flag_hash = true;
	blk->node = NULL;
	h_new->blk = blk;
	h_new->found = (found ? 1 : 0);

	/* insert the block into the hash table */
hsh_add_restart:
	h_iter = state->htbl[h_val & _BPF_HASH_MASK];
	if (h_iter != NULL) {
		do {
			if ((h_iter->blk->hash == h_val) &&
			    (_BLK_MSZE(h_iter->blk) == _BLK_MSZE(blk)) &&
			    (memcmp(h_iter->blk->blks, blk->blks,
				    _BLK_MSZE(blk)) == 0)) {
				/* duplicate block */
				free(h_new);

				/* store the duplicate block */
				b_iter = h_iter->blk;
				while (b_iter->hash_nxt != NULL)
					b_iter = b_iter->hash_nxt;
				b_iter->hash_nxt = blk;

				/* in some cases we want to return the
				 * duplicate block */
				if (found) {
					blk->flag_dup = true;
					return 0;
				}

				/* update the priority if needed */
				if (h_iter->blk->priority < blk->priority)
					h_iter->blk->priority = blk->priority;

				/* try to save some memory */
				free(blk->blks);
				blk->blks = h_iter->blk->blks;
				blk->flag_unique = false;

				*blk_p = h_iter->blk;
				return 0;
			} else if (h_iter->blk->hash == h_val) {
				/* hash collision */
				if ((h_val >> 32) == 0xffffffff) {
					/* overflow */
					blk->flag_hash = false;
					blk->hash = 0;
					free(h_new);
					return -EFAULT;
				}
				h_val += ((uint64_t)1 << 32);
				h_new->blk->hash = h_val;

				/* restart at the beginning of the bucket */
				goto hsh_add_restart;
			} else {
				/* no match, move along */
				h_prev = h_iter;
				h_iter = h_iter->next;
			}
		} while (h_iter != NULL);
		h_prev->next = h_new;
	} else
		state->htbl[h_val & _BPF_HASH_MASK] = h_new;

	return 0;
}

/**
 * Remove an entry from the hash table
 * @param state the BPF state
 * @param h_val the hash value
 *
 * Remove an entry from the hash table and return it to the caller, NULL is
 * returned if the entry can not be found.
 *
 */
static struct bpf_blk *_hsh_remove(struct bpf_state *state, uint64_t h_val)
{
	unsigned int bkt = h_val & _BPF_HASH_MASK;
	struct bpf_blk *blk;
	struct bpf_hash_bkt *h_iter, *h_prev = NULL;

	h_iter = state->htbl[bkt];
	while (h_iter != NULL) {
		if (h_iter->blk->hash == h_val) {
			if (h_prev != NULL)
				h_prev->next = h_iter->next;
			else
				state->htbl[bkt] = h_iter->next;
			blk = h_iter->blk;
			free(h_iter);
			return blk;
		}
		h_prev = h_iter;
		h_iter =  h_iter->next;
	}

	return NULL;
}

/**
 * Find and return a hash bucket
 * @param state the BPF state
 * @param h_val the hash value
 *
 * Find the entry associated with the given hash value and return it to the
 * caller, NULL is returned if the entry can not be found.  This function
 * should not be called directly; use _hsh_find() and _hsh_find_once() instead.
 *
 */
static struct bpf_hash_bkt *_hsh_find_bkt(const struct bpf_state *state,
					  uint64_t h_val)
{
	struct bpf_hash_bkt *h_iter;

	h_iter = state->htbl[h_val & _BPF_HASH_MASK];
	while (h_iter != NULL) {
		if (h_iter->blk->hash == h_val)
			return h_iter;
		h_iter = h_iter->next;
	}

	return NULL;
}

/**
 * Find and only return an entry in the hash table once
 * @param state the BPF state
 * @param h_val the hash value
 *
 * Find the entry associated with the given hash value and return it to the
 * caller if it has not be returned previously by this function; returns NULL
 * if the entry can not be found or has already been returned in a previous
 * call.
 *
 */
static struct bpf_blk *_hsh_find_once(const struct bpf_state *state,
				      uint64_t h_val)
{
	struct bpf_hash_bkt *h_iter;

	h_iter = _hsh_find_bkt(state, h_val);
	if (h_iter == NULL || h_iter->found != 0)
		return NULL;
	h_iter->found = 1;
	return h_iter->blk;
}

/**
 * Finds an entry in the hash table
 * @param state the BPF state
 * @param h_val the hash value
 *
 * Find the entry associated with the given hash value and return it to the
 * caller, NULL is returned if the entry can not be found.
 *
 */
static struct bpf_blk *_hsh_find(const struct bpf_state *state, uint64_t h_val)
{
	struct bpf_hash_bkt *h_iter;

	h_iter = _hsh_find_bkt(state, h_val);
	if (h_iter == NULL)
		return NULL;
	return h_iter->blk;
}

/**
 * Generate a BPF action instruction
 * @param state the BPF state
 * @param blk the BPF instruction block, or NULL
 * @param action the desired action
 *
 * Generate a BPF action instruction and append it to the given instruction
 * block.  Returns a pointer to the instruction block on success, NULL on
 * failure.
 *
 */
static struct bpf_blk *_gen_bpf_action(struct bpf_state *state,
				       struct bpf_blk *blk, uint32_t action)
{
	struct bpf_instr instr;

	_BPF_INSTR(instr, _BPF_OP(state->arch, BPF_RET),
		   _BPF_JMP_NO, _BPF_JMP_NO, _BPF_K(state->arch, action));
	return _blk_append(state, blk, &instr);
}

/**
 * Generate a BPF action instruction and insert it into the hash table
 * @param state the BPF state
 * @param action the desired action
 *
 * Generate a BPF action instruction and insert it into the hash table.
 * Returns a pointer to the instruction block on success, NULL on failure.
 *
 */
static struct bpf_blk *_gen_bpf_action_hsh(struct bpf_state *state,
					   uint32_t action)
{
	struct bpf_blk *blk;

	blk = _gen_bpf_action(state, NULL, action);
	if (blk == NULL)
		return NULL;
	if (_hsh_add(state, &blk, 0) < 0) {
		_blk_free(state, blk);
		return NULL;
	}

	return blk;
}

/**
 * Generate a BPF instruction block for a given chain node
 * @param state the BPF state
 * @param node the filter chain node
 * @param a_state the accumulator state
 *
 * Generate BPF instructions to execute the filter for the given chain node.
 * Returns a pointer to the instruction block on success, NULL on failure.
 *
 */
static struct bpf_blk *_gen_bpf_node(struct bpf_state *state,
				     const struct db_arg_chain_tree *node,
				     struct acc_state *a_state)
{
	int32_t acc_offset;
	uint32_t acc_mask;
	uint64_t act_t_hash = 0, act_f_hash = 0;
	struct bpf_blk *blk = NULL, *b_act;
	struct bpf_instr instr;
	struct acc_state a_state_orig = *a_state;

	/* generate the action blocks */
	if (node->act_t_flg) {
		b_act = _gen_bpf_action_hsh(state, node->act_t);
		if (b_act == NULL)
			goto node_failure;
		act_t_hash = b_act->hash;
	}
	if (node->act_f_flg) {
		b_act = _gen_bpf_action_hsh(state, node->act_f);
		if (b_act == NULL)
			goto node_failure;
		act_f_hash = b_act->hash;
	}

	/* check the accumulator state */
	acc_offset = node->arg_offset;
	acc_mask = node->mask;
	if (acc_offset < 0)
		goto node_failure;
	if ((acc_offset != a_state->offset) ||
	    ((acc_mask & a_state->mask) != acc_mask)) {
		/* reload the accumulator */
		a_state->offset = acc_offset;
		a_state->mask = ARG_MASK_MAX;
		_BPF_INSTR(instr, _BPF_OP(state->arch, BPF_LD + BPF_ABS),
			   _BPF_JMP_NO, _BPF_JMP_NO,
			   _BPF_K(state->arch, acc_offset));
		blk = _blk_append(state, blk, &instr);
		if (blk == NULL)
			goto node_failure;
	}
	if (acc_mask != a_state->mask) {
		/* apply the bitmask */
		a_state->mask = acc_mask;
		_BPF_INSTR(instr, _BPF_OP(state->arch, BPF_ALU + BPF_AND),
			   _BPF_JMP_NO, _BPF_JMP_NO,
			   _BPF_K(state->arch, acc_mask));
		blk = _blk_append(state, blk, &instr);
		if (blk == NULL)
			goto node_failure;
	}

	/* check the accumulator against the datum */
	switch (node->op) {
	case SCMP_CMP_MASKED_EQ:
	case SCMP_CMP_EQ:
		_BPF_INSTR(instr, _BPF_OP(state->arch, BPF_JMP + BPF_JEQ),
			   _BPF_JMP_NO, _BPF_JMP_NO,
			   _BPF_K(state->arch, node->datum));
		break;
	case SCMP_CMP_GT:
		_BPF_INSTR(instr, _BPF_OP(state->arch, BPF_JMP + BPF_JGT),
			   _BPF_JMP_NO, _BPF_JMP_NO,
			   _BPF_K(state->arch, node->datum));
		break;
	case SCMP_CMP_GE:
		_BPF_INSTR(instr, _BPF_OP(state->arch, BPF_JMP + BPF_JGE),
			   _BPF_JMP_NO, _BPF_JMP_NO,
			   _BPF_K(state->arch, node->datum));
		break;
	case SCMP_CMP_NE:
	case SCMP_CMP_LT:
	case SCMP_CMP_LE:
		/* if we hit here it means the filter db isn't correct */
	default:
		/* fatal error, we should never get here */
		goto node_failure;
	}

	/* fixup the jump targets */
	if (node->nxt_t != NULL)
		instr.jt = _BPF_JMP_DB(node->nxt_t);
	else if (node->act_t_flg)
		instr.jt = _BPF_JMP_HSH(act_t_hash);
	else
		instr.jt = _BPF_JMP_NXT(0);
	if (node->nxt_f != NULL)
		instr.jf = _BPF_JMP_DB(node->nxt_f);
	else if (node->act_f_flg)
		instr.jf = _BPF_JMP_HSH(act_f_hash);
	else
		instr.jf = _BPF_JMP_NXT(0);
	blk = _blk_append(state, blk, &instr);
	if (blk == NULL)
		goto node_failure;

	blk->node = node;
	blk->acc_state = a_state_orig;
	return blk;

node_failure:
	_blk_free(state, blk);
	return NULL;
}

/**
 * Resolve the jump targets in a BPF instruction block
 * @param state the BPF state
 * @param sys the syscall filter
 * @param blk the BPF instruction block
 * @param nxt_jump the jump to fallthrough to at the end of the level
 *
 * Resolve the jump targets in a BPF instruction block generated by the
 * _gen_bpf_chain_lvl() function and adds the resulting block to the hash
 * table.  Returns a pointer to the new instruction block on success, NULL on
 * failure.
 *
 */
static struct bpf_blk *_gen_bpf_chain_lvl_res(struct bpf_state *state,
					      const struct db_sys_list *sys,
					      struct bpf_blk *blk,
					      const struct bpf_jump *nxt_jump)
{
	int rc;
	unsigned int iter;
	struct bpf_blk *b_new;
	struct bpf_instr *i_iter;
	struct db_arg_chain_tree *node;

	if (blk->flag_hash)
		return blk;

	/* convert TGT_PTR_DB to TGT_PTR_HSH references */
	for (iter = 0; iter < blk->blk_cnt; iter++) {
		i_iter = &blk->blks[iter];
		switch (i_iter->jt.type) {
		case TGT_NONE:
		case TGT_IMM:
		case TGT_PTR_HSH:
			/* ignore these jump types */
			break;
		case TGT_PTR_BLK:
			b_new = _gen_bpf_chain_lvl_res(state, sys,
						       i_iter->jt.tgt.blk,
						       nxt_jump);
			if (b_new == NULL)
				return NULL;
			i_iter->jt = _BPF_JMP_HSH(b_new->hash);
			break;
		case TGT_PTR_DB:
			node = (struct db_arg_chain_tree *)i_iter->jt.tgt.db;
			b_new = _gen_bpf_chain(state, sys, node,
					       nxt_jump, &blk->acc_state);
			if (b_new == NULL)
				return NULL;
			i_iter->jt = _BPF_JMP_HSH(b_new->hash);
			break;
		default:
			/* we should not be here */
			return NULL;
		}
		switch (i_iter->jf.type) {
		case TGT_NONE:
		case TGT_IMM:
		case TGT_PTR_HSH:
			/* ignore these jump types */
			break;
		case TGT_PTR_BLK:
			b_new = _gen_bpf_chain_lvl_res(state, sys,
						       i_iter->jf.tgt.blk,
						       nxt_jump);
			if (b_new == NULL)
				return NULL;
			i_iter->jf = _BPF_JMP_HSH(b_new->hash);
			break;
		case TGT_PTR_DB:
			node = (struct db_arg_chain_tree *)i_iter->jf.tgt.db;
			b_new = _gen_bpf_chain(state, sys, node,
					       nxt_jump, &blk->acc_state);
			if (b_new == NULL)
				return NULL;
			i_iter->jf = _BPF_JMP_HSH(b_new->hash);
			break;
		default:
			/* we should not be here */
			return NULL;
		}
		switch (i_iter->k.type) {
		case TGT_NONE:
		case TGT_K:
		case TGT_PTR_HSH:
			/* ignore these jump types */
			break;
		default:
			/* we should not be here */
			return NULL;
		}
	}

	/* insert the block into the hash table */
	rc = _hsh_add(state, &blk, 0);
	if (rc < 0)
		return NULL;

	return blk;
}

/**
 * Generates the BPF instruction blocks for a given filter chain
 * @param state the BPF state
 * @param sys the syscall filter
 * @param chain the filter chain
 * @param nxt_jump the jump to fallthrough to at the end of the level
 * @param a_state the accumulator state
 *
 * Generate the BPF instruction blocks for the given filter chain and return
 * a pointer to the first block on success; returns NULL on failure.
 *
 */
static struct bpf_blk *_gen_bpf_chain(struct bpf_state *state,
				      const struct db_sys_list *sys,
				      const struct db_arg_chain_tree *chain,
				      const struct bpf_jump *nxt_jump,
				      struct acc_state *a_state)
{
	struct bpf_blk *b_head = NULL, *b_tail = NULL;
	struct bpf_blk *b_prev, *b_next, *b_iter;
	struct bpf_instr *i_iter;
	const struct db_arg_chain_tree *c_iter;
	unsigned int iter;
	struct bpf_jump nxt_jump_tmp;

	if (chain == NULL) {
		b_head = _gen_bpf_action(state, NULL, sys->action);
		if (b_head == NULL)
			goto chain_failure;
		b_tail = b_head;
	} else {
		/* find the starting node of the level */
		c_iter = chain;
		while (c_iter->lvl_prv != NULL)
			c_iter = c_iter->lvl_prv;

		/* build all of the blocks for this level */
		do {
			b_iter = _gen_bpf_node(state, c_iter, a_state);
			if (b_iter == NULL)
				goto chain_failure;
			if (b_head != NULL) {
				b_iter->lvl_prv = b_tail;
				b_tail->lvl_nxt = b_iter;
				b_tail = b_iter;
			} else {
				b_head = b_iter;
				b_tail = b_iter;
			}
			c_iter = c_iter->lvl_nxt;
		} while (c_iter != NULL);

		/* resolve the TGT_NXT jumps */
		b_iter = b_head;
		do {
			b_next = b_iter->lvl_nxt;
			for (iter = 0; iter < b_iter->blk_cnt; iter++) {
				i_iter = &b_iter->blks[iter];
				if (i_iter->jt.type == TGT_NXT) {
					if (i_iter->jt.tgt.nxt != 0)
						goto chain_failure;
					if (b_next == NULL)
						i_iter->jt = *nxt_jump;
					else
						i_iter->jt =
							_BPF_JMP_BLK(b_next);
				}
				if (i_iter->jf.type == TGT_NXT) {
					if (i_iter->jf.tgt.nxt != 0)
						goto chain_failure;
					if (b_next == NULL)
						i_iter->jf = *nxt_jump;
					else
						i_iter->jf =
							_BPF_JMP_BLK(b_next);
				}
			}
			b_iter = b_next;
		} while (b_iter != NULL);
	}

	/* resolve all of the blocks */
	memset(&nxt_jump_tmp, 0, sizeof(nxt_jump_tmp));
	b_iter = b_tail;
	do {
		/* b_iter may change after resolving, so save the linkage */
		b_prev = b_iter->lvl_prv;
		b_next = b_iter->lvl_nxt;

		nxt_jump_tmp = _BPF_JMP_BLK(b_next);
		b_iter = _gen_bpf_chain_lvl_res(state, sys, b_iter,
						(b_next == NULL ?
						 nxt_jump :
						 &nxt_jump_tmp));
		if (b_iter == NULL)
			goto chain_failure;

		/* restore the block linkage on this level */
		if (b_prev != NULL)
			b_prev->lvl_nxt = b_iter;
		b_iter->lvl_prv = b_prev;
		b_iter->lvl_nxt = b_next;
		if (b_next != NULL)
			b_next->lvl_prv = b_iter;
		if (b_iter->lvl_prv == NULL)
			b_head = b_iter;

		b_iter = b_prev;
	} while (b_iter != NULL);

	return b_head;

chain_failure:
	while (b_head != NULL) {
		b_iter = b_head;
		b_head = b_iter->lvl_nxt;
		_blk_free(state, b_iter);
	}
	return NULL;
}

/**
 * Generate the BPF instruction blocks for a given syscall
 * @param state the BPF state
 * @param sys the syscall filter DB entry
 * @param nxt_hash the hash value of the next syscall filter DB entry
 * @param acc_reset accumulator reset flag
 *
 * Generate the BPF instruction blocks for the given syscall filter and return
 * a pointer to the first block on success; returns NULL on failure.  It is
 * important to note that the block returned has not been added to the hash
 * table, however, any linked/referenced blocks have been added to the hash
 * table.
 *
 */
static struct bpf_blk *_gen_bpf_syscall(struct bpf_state *state,
					const struct db_sys_list *sys,
					uint64_t nxt_hash,
					bool acc_reset)
{
	int rc;
	struct bpf_instr instr;
	struct bpf_blk *blk_c, *blk_s = NULL;
	struct bpf_jump def_jump;
	struct acc_state a_state;

	/* we do the memset before the assignment to keep valgrind happy */
	memset(&def_jump, 0, sizeof(def_jump));
	def_jump = _BPF_JMP_HSH(state->def_hsh);

	/* setup the accumulator state */
	if (acc_reset) {
		_BPF_INSTR(instr, _BPF_OP(state->arch, BPF_LD + BPF_ABS),
			   _BPF_JMP_NO, _BPF_JMP_NO,
			   _BPF_SYSCALL(state->arch));
		blk_s = _blk_append(state, NULL, &instr);
		if (blk_s == NULL)
			return NULL;
		a_state.offset = _BPF_OFFSET_SYSCALL;
		a_state.mask = ARG_MASK_MAX;
	} else {
		/* set the accumulator state to an unknown value */
		a_state.offset = -1;
		a_state.mask = ARG_MASK_MAX;
	}

	/* generate the argument chains */
	blk_c = _gen_bpf_chain(state, sys, sys->chains, &def_jump, &a_state);
	if (blk_c == NULL) {
		_blk_free(state, blk_s);
		return NULL;
	}

	/* syscall check */
	_BPF_INSTR(instr, _BPF_OP(state->arch, BPF_JMP + BPF_JEQ),
		   _BPF_JMP_HSH(blk_c->hash), _BPF_JMP_HSH(nxt_hash),
		   _BPF_K(state->arch, sys->num));
	blk_s = _blk_append(state, blk_s, &instr);
	if (blk_s == NULL)
		return NULL;
	blk_s->priority = sys->priority;

	/* add to the hash table */
	rc = _hsh_add(state, &blk_s, 1);
	if (rc < 0) {
		_blk_free(state, blk_s);
		return NULL;
	}

	return blk_s;
}

/**
 * Generate the BPF instruction blocks for a given filter/architecture
 * @param state the BPF state
 * @param db the filter DB
 * @param db_secondary the secondary DB
 *
 * Generate the BPF instruction block for the given filter DB(s)/architecture(s)
 * and return a pointer to the block on succes, NULL on failure.  The resulting
 * block assumes that the architecture token has already been loaded into the
 * BPF accumulator.
 *
 */
static struct bpf_blk *_gen_bpf_arch(struct bpf_state *state,
				     const struct db_filter *db,
				     const struct db_filter *db_secondary)
{
	int rc;
	unsigned int blk_cnt = 0;
	bool acc_reset;
	struct bpf_instr instr;
	struct db_sys_list *s_head = NULL, *s_tail = NULL, *s_iter, *s_iter_b;
	struct bpf_blk *b_head = NULL, *b_tail = NULL, *b_iter, *b_new;

	state->arch = db->arch;

	/* sort the syscall list */
	db_list_foreach(s_iter, db->syscalls) {
		if (s_head != NULL) {
			s_iter_b = s_head;
			while ((s_iter_b->pri_nxt != NULL) &&
			       (s_iter->priority <= s_iter_b->priority))
				s_iter_b = s_iter_b->pri_nxt;

			if (s_iter->priority > s_iter_b->priority) {
				s_iter->pri_prv = s_iter_b->pri_prv;
				s_iter->pri_nxt = s_iter_b;
				if (s_iter_b == s_head) {
					s_head->pri_prv = s_iter;
					s_head = s_iter;
				} else {
					s_iter->pri_prv->pri_nxt = s_iter;
					s_iter->pri_nxt->pri_prv = s_iter;
				}
			} else {
				s_iter->pri_prv = s_tail;
				s_iter->pri_nxt = NULL;
				s_iter->pri_prv->pri_nxt = s_iter;
				s_tail = s_iter;
			}
		} else {
			s_head = s_iter;
			s_tail = s_iter;
			s_head->pri_prv = NULL;
			s_head->pri_nxt = NULL;
		}
	}
	if (db_secondary != NULL) {
		db_list_foreach(s_iter, db_secondary->syscalls) {
			if (s_head != NULL) {
				s_iter_b = s_head;
				while ((s_iter_b->pri_nxt != NULL) &&
				       (s_iter->priority <= s_iter_b->priority))
					s_iter_b = s_iter_b->pri_nxt;

				if (s_iter->priority > s_iter_b->priority) {
					s_iter->pri_prv = s_iter_b->pri_prv;
					s_iter->pri_nxt = s_iter_b;
					if (s_iter_b == s_head) {
						s_head->pri_prv = s_iter;
						s_head = s_iter;
					} else {
						s_iter->pri_prv->pri_nxt =
							s_iter;
						s_iter->pri_nxt->pri_prv =
							s_iter;
					}
				} else {
					s_iter->pri_prv = s_tail;
					s_iter->pri_nxt = NULL;
					s_iter->pri_prv->pri_nxt = s_iter;
					s_tail = s_iter;
				}
			} else {
				s_head = s_iter;
				s_tail = s_iter;
				s_head->pri_prv = NULL;
				s_head->pri_nxt = NULL;
			}
		}
	}

	if ((state->arch->token == SCMP_ARCH_X86_64 ||
	     state->arch->token == SCMP_ARCH_X32) && (db_secondary == NULL))
		acc_reset = false;
	else
		acc_reset = true;

	/* create the syscall filters and add them to block list group */
	for (s_iter = s_tail; s_iter != NULL; s_iter = s_iter->pri_prv) {
		if (!s_iter->valid)
			continue;

		/* build the syscall filter */
		b_new = _gen_bpf_syscall(state, s_iter,
					 (b_head == NULL ?
					  state->def_hsh : b_head->hash),
					 (s_iter == s_head ?
					  acc_reset : false));
		if (b_new == NULL)
			goto arch_failure;

		/* add the filter to the list head */
		b_new->prev = NULL;
		b_new->next = b_head;
		if (b_tail != NULL) {
			b_head->prev = b_new;
			b_head = b_new;
		} else {
			b_head = b_new;
			b_tail = b_head;
		}

		if (b_tail->next != NULL)
			b_tail = b_tail->next;
		blk_cnt++;
	}

	/* additional ABI filtering */
	if ((state->arch->token == SCMP_ARCH_X86_64 ||
	     state->arch->token == SCMP_ARCH_X32) && (db_secondary == NULL)) {
		_BPF_INSTR(instr, _BPF_OP(state->arch, BPF_LD + BPF_ABS),
			   _BPF_JMP_NO, _BPF_JMP_NO, _BPF_SYSCALL(state->arch));
		b_new = _blk_append(state, NULL, &instr);
		if (b_new == NULL)
			goto arch_failure;
		if (state->arch->token == SCMP_ARCH_X86_64) {
			/* filter out x32 */
			_BPF_INSTR(instr,
				   _BPF_OP(state->arch, BPF_JMP + BPF_JGE),
				   _BPF_JMP_NXT(blk_cnt++), _BPF_JMP_NO,
				   _BPF_K(state->arch, X32_SYSCALL_BIT));
			if (b_head != NULL)
				instr.jf = _BPF_JMP_HSH(b_head->hash);
			else
				instr.jf = _BPF_JMP_HSH(state->def_hsh);
		} else if (state->arch->token == SCMP_ARCH_X32) {
			/* filter out x86_64 */
			_BPF_INSTR(instr,
				   _BPF_OP(state->arch, BPF_JMP + BPF_JGE),
				   _BPF_JMP_NO, _BPF_JMP_NXT(blk_cnt++),
				   _BPF_K(state->arch, X32_SYSCALL_BIT));
			if (b_head != NULL)
				instr.jt = _BPF_JMP_HSH(b_head->hash);
			else
				instr.jt = _BPF_JMP_HSH(state->def_hsh);
		} else
			/* we should never get here */
			goto arch_failure;
		b_new = _blk_append(state, b_new, &instr);
		if (b_new == NULL)
			goto arch_failure;
		b_new->next = b_head;
		if (b_head != NULL)
			b_head->prev = b_new;
		b_head = b_new;
		rc = _hsh_add(state, &b_head, 1);
		if (rc < 0)
			goto arch_failure;
	}

	/* do the ABI/architecture check */
	_BPF_INSTR(instr, _BPF_OP(state->arch, BPF_JMP + BPF_JEQ),
		   _BPF_JMP_NO, _BPF_JMP_NXT(blk_cnt++),
		   _BPF_K(state->arch, state->arch->token_bpf));
	if (b_head != NULL)
		instr.jt = _BPF_JMP_HSH(b_head->hash);
	else
		instr.jt = _BPF_JMP_HSH(state->def_hsh);
	b_new = _blk_append(state, NULL, &instr);
	if (b_new == NULL)
		goto arch_failure;
	b_new->next = b_head;
	if (b_head != NULL)
		b_head->prev = b_new;
	b_head = b_new;
	rc = _hsh_add(state, &b_head, 1);
	if (rc < 0)
		goto arch_failure;

	state->arch = NULL;
	return b_head;

arch_failure:
	/* NOTE: we do the cleanup here and not just return an error as all of
	 * the instruction blocks may not be added to the hash table when we
	 * hit an error */
	state->arch = NULL;
	b_iter = b_head;
	while (b_iter != NULL) {
		b_new = b_iter->next;
		_blk_free(state, b_iter);
		b_iter = b_new;
	}
	return NULL;
}

/**
 * Find the target block for the "next" jump
 * @param blk the instruction block
 * @param nxt the next offset
 *
 * Find the target block for the TGT_NXT jump using the given offset.  Returns
 * a pointer to the target block on success or NULL on failure.
 *
 */
static struct bpf_blk *_gen_bpf_find_nxt(const struct bpf_blk *blk,
					 unsigned int nxt)
{
	struct bpf_blk *iter = blk->next;

	for (; (iter != NULL) && (nxt > 0); nxt--)
		iter = iter->next;

	return iter;
}

/**
 * Manage jumps to return instructions
 * @param state the BPF state
 * @param blk the instruction block to check
 * @param offset the instruction offset into the instruction block
 * @param blk_ret the return instruction block
 *
 * Using the given block and instruction offset, calculate the jump distance
 * between the jumping instruction and return instruction block.  If the jump
 * distance is too great, duplicate the return instruction to reduce the
 * distance to the maximum value.  Returns 1 if a long jump was added, zero if
 * the existing jump is valid, and negative values on failure.
 *
 */
static int _gen_bpf_build_jmp_ret(struct bpf_state *state,
				  struct bpf_blk *blk, unsigned int offset,
				  struct bpf_blk *blk_ret)
{
	unsigned int j_len;
	uint64_t tgt_hash = blk_ret->hash;
	struct bpf_blk *b_jmp, *b_new;

	/* calculate the jump distance */
	j_len = blk->blk_cnt - (offset + 1);
	b_jmp = blk->next;
	while (b_jmp != NULL && b_jmp != blk_ret && j_len < _BPF_JMP_MAX_RET) {
		j_len += b_jmp->blk_cnt;
		b_jmp = b_jmp->next;
	}
	if (b_jmp == NULL)
		return -EFAULT;
	if (j_len <= _BPF_JMP_MAX_RET && b_jmp == blk_ret)
		return 0;

	/* we need a closer return instruction, see if one already exists */
	j_len = blk->blk_cnt - (offset + 1);
	b_jmp = blk->next;
	while (b_jmp != NULL && b_jmp->hash != tgt_hash &&
	       j_len < _BPF_JMP_MAX_RET) {
		j_len += b_jmp->blk_cnt;
		b_jmp = b_jmp->next;
	}
	if (b_jmp == NULL)
		return -EFAULT;
	if (j_len <= _BPF_JMP_MAX_RET && b_jmp->hash == tgt_hash)
		return 0;

	/* we need to insert a new return instruction - create one */
	b_new = _gen_bpf_action(state, NULL, blk_ret->blks[0].k.tgt.imm_k);
	if (b_new == NULL)
		return -EFAULT;

	/* NOTE - we need to be careful here, we're giving the block a hash
	 *	  value (this is a sneaky way to ensure we leverage the
	 *	  inserted long jumps as much as possible) but we never add the
	 *	  block to the hash table so it won't get cleaned up
	 *	  automatically */
	b_new->hash = tgt_hash;

	/* insert the jump after the current jumping block */
	b_new->prev = blk;
	b_new->next = blk->next;
	blk->next->prev = b_new;
	blk->next = b_new;

	return 1;
}

/**
 * Manage jump lengths by duplicating and adding jumps if needed
 * @param state the BPF state
 * @param tail the tail of the instruction block list
 * @param blk the instruction block to check
 * @param offset the instruction offset into the instruction block
 * @param tgt_hash the hash of the jump destination block
 *
 * Using the given block and instruction offset, calculate the jump distance
 * between the jumping instruction and the destination.  If the jump distance
 * is too great, add a long jump instruction to reduce the distance to a legal
 * value.  Returns 1 if a new instruction was added, zero if the existing jump
 * is valid, and negative values on failure.
 *
 */
static int _gen_bpf_build_jmp(struct bpf_state *state,
			      struct bpf_blk *tail,
			      struct bpf_blk *blk, unsigned int offset,
			      uint64_t tgt_hash)
{
	int rc;
	unsigned int jmp_len;
	struct bpf_instr instr;
	struct bpf_blk *b_new, *b_jmp, *b_tgt;

	/* find the jump target */
	b_tgt = tail;
	while (b_tgt != blk && b_tgt->hash != tgt_hash)
		b_tgt = b_tgt->prev;
	if (b_tgt == blk)
		return -EFAULT;

	if (b_tgt->blk_cnt == 1 &&
	    b_tgt->blks[0].op == _BPF_OP(state->arch, BPF_RET)) {
		rc = _gen_bpf_build_jmp_ret(state, blk, offset, b_tgt);
		if (rc == 1)
			return 1;
		else if (rc < 0)
			return rc;
	}

	/* calculate the jump distance */
	jmp_len = blk->blk_cnt - (offset + 1);
	b_jmp = blk->next;
	while (b_jmp != NULL && b_jmp != b_tgt && jmp_len < _BPF_JMP_MAX) {
		jmp_len += b_jmp->blk_cnt;
		b_jmp = b_jmp->next;
	}
	if (b_jmp == NULL)
		return -EFAULT;
	if (jmp_len <= _BPF_JMP_MAX && b_jmp == b_tgt)
		return 0;

	/* we need a long jump, see if one already exists */
	jmp_len = blk->blk_cnt - (offset + 1);
	b_jmp = blk->next;
	while (b_jmp != NULL && b_jmp->hash != tgt_hash &&
	       jmp_len < _BPF_JMP_MAX) {
		jmp_len += b_jmp->blk_cnt;
		b_jmp = b_jmp->next;
	}
	if (b_jmp == NULL)
		return -EFAULT;
	if (jmp_len <= _BPF_JMP_MAX && b_jmp->hash == tgt_hash)
		return 0;

	/* we need to insert a long jump - create one */
	_BPF_INSTR(instr, _BPF_OP(state->arch, BPF_JMP + BPF_JA),
		   _BPF_JMP_NO, _BPF_JMP_NO, _BPF_JMP_HSH(tgt_hash));
	b_new = _blk_append(state, NULL, &instr);
	if (b_new == NULL)
		return -EFAULT;

	/* NOTE - we need to be careful here, we're giving the block a hash
	 *	  value (this is a sneaky way to ensure we leverage the
	 *	  inserted long jumps as much as possible) but we never add the
	 *	  block to the hash table so it won't get cleaned up
	 *	  automatically */
	b_new->hash = tgt_hash;

	/* insert the jump after the current jumping block */
	b_new->prev = blk;
	b_new->next = blk->next;
	blk->next->prev = b_new;
	blk->next = b_new;

	return 1;
}

/**
 * Generate the BPF program for the given filter collection
 * @param state the BPF state
 * @param col the filter collection
 *
 * Generate the BPF program for the given filter collection.  Returns zero on
 * success, negative values on failure.
 *
 */
static int _gen_bpf_build_bpf(struct bpf_state *state,
			      const struct db_filter_col *col)
{
	int rc;
	int iter;
	uint64_t h_val;
	unsigned int res_cnt;
	unsigned int jmp_len;
	int arch_x86_64 = -1, arch_x32 = -1;
	struct bpf_instr instr;
	struct bpf_instr *i_iter;
	struct bpf_blk *b_badarch, *b_default;
	struct bpf_blk *b_head = NULL, *b_tail = NULL, *b_iter, *b_new, *b_jmp;
	struct db_filter *db_secondary = NULL;
	struct arch_def pseudo_arch;

	if (col->filter_cnt == 0)
		return -EINVAL;

	/* create a fake architecture definition for use in the early stages */
	memset(&pseudo_arch, 0, sizeof(pseudo_arch));
	pseudo_arch.endian = col->endian;
	state->arch = &pseudo_arch;

	/* generate the badarch action */
	b_badarch = _gen_bpf_action(state, NULL, state->attr->act_badarch);
	if (b_badarch == NULL)
		return -ENOMEM;
	rc = _hsh_add(state, &b_badarch, 1);
	if (rc < 0)
		return rc;

	/* generate the default action */
	b_default = _gen_bpf_action(state, NULL, state->attr->act_default);
	if (b_default == NULL)
		return -ENOMEM;
	rc = _hsh_add(state, &b_default, 0);
	if (rc < 0)
		return rc;
	state->def_hsh = b_default->hash;

	/* load the architecture token/number */
	_BPF_INSTR(instr, _BPF_OP(state->arch, BPF_LD + BPF_ABS),
		   _BPF_JMP_NO, _BPF_JMP_NO,
		   _BPF_K(state->arch, offsetof(struct seccomp_data, arch)));
	b_head = _blk_append(state, NULL, &instr);
	if (b_head == NULL)
		return -ENOMEM;
	rc = _hsh_add(state, &b_head, 1);
	if (rc < 0)
		return rc;
	b_tail = b_head;

	/* generate the per-architecture filters */
	for (iter = 0; iter < col->filter_cnt; iter++) {
		if (col->filters[iter]->arch->token == SCMP_ARCH_X86_64)
			arch_x86_64 = iter;
		if (col->filters[iter]->arch->token == SCMP_ARCH_X32)
			arch_x32 = iter;
	}
	for (iter = 0; iter < col->filter_cnt; iter++) {
		/* figure out the secondary arch filter mess */
		if (iter == arch_x86_64) {
			if (arch_x32 > iter)
				db_secondary = col->filters[arch_x32];
			else if (arch_x32 >= 0)
				continue;
		} else if (iter == arch_x32) {
			if (arch_x86_64 > iter)
				db_secondary = col->filters[arch_x86_64];
			else if (arch_x86_64 >= 0)
				continue;
		} else
			db_secondary = NULL;

		/* create the filter for the architecture(s) */
		b_new = _gen_bpf_arch(state, col->filters[iter], db_secondary);
		if (b_new == NULL)
			return -ENOMEM;
		b_new->prev = b_tail;
		b_tail->next = b_new;
		b_tail = b_new;
		while (b_tail->next != NULL)
			b_tail = b_tail->next;
	}

	/* add a badarch action to the end */
	b_badarch->prev = b_tail;
	b_badarch->next = NULL;
	b_tail->next = b_badarch;
	b_tail = b_badarch;

	/* reset the state to the pseudo_arch for the final resolution */
	state->arch = &pseudo_arch;

	/* resolve any TGT_NXT jumps at the top level */
	b_iter = b_head;
	do {
		for (iter = 0; iter < b_iter->blk_cnt; iter++) {
			i_iter = &b_iter->blks[iter];
			if (i_iter->jt.type == TGT_NXT) {
				b_jmp = _gen_bpf_find_nxt(b_iter,
							  i_iter->jt.tgt.nxt);
				if (b_jmp == NULL)
					return -EFAULT;
				i_iter->jt = _BPF_JMP_HSH(b_jmp->hash);
			}
			if (i_iter->jf.type == TGT_NXT) {
				b_jmp = _gen_bpf_find_nxt(b_iter,
							  i_iter->jf.tgt.nxt);
				if (b_jmp == NULL)
					return -EFAULT;
				i_iter->jf = _BPF_JMP_HSH(b_jmp->hash);
			}
			/* we shouldn't need to worry about a TGT_NXT in k */
		}
		b_iter = b_iter->next;
	} while (b_iter != NULL && b_iter->next != NULL);

	/* pull in all of the TGT_PTR_HSH jumps, one layer at a time */
	b_iter = b_tail;
	do {
		b_jmp = NULL;
		/* look for jumps - backwards (shorter jumps) */
		for (iter = b_iter->blk_cnt - 1;
		     (iter >= 0) && (b_jmp == NULL);
		     iter--) {
			i_iter = &b_iter->blks[iter];
			if (i_iter->jt.type == TGT_PTR_HSH)
				b_jmp = _hsh_find_once(state,
						       i_iter->jt.tgt.hash);
			if (b_jmp == NULL && i_iter->jf.type == TGT_PTR_HSH)
				b_jmp = _hsh_find_once(state,
						       i_iter->jf.tgt.hash);
			if (b_jmp == NULL && i_iter->k.type == TGT_PTR_HSH)
				b_jmp = _hsh_find_once(state,
						       i_iter->k.tgt.hash);
			if (b_jmp != NULL) {
				/* insert the new block after this block */
				b_jmp->prev = b_iter;
				b_jmp->next = b_iter->next;
				b_iter->next = b_jmp;
				if (b_jmp->next)
					b_jmp->next->prev = b_jmp;
			}
		}
		if (b_jmp != NULL) {
			while (b_tail->next != NULL)
				b_tail = b_tail->next;
			b_iter = b_tail;
		} else
			b_iter = b_iter->prev;
	} while (b_iter != NULL);

	/* NOTE - from here to the end of the function we need to fail via the
	 *	  the build_bpf_free_blks label, not just return an error; see
	 *	  the _gen_bpf_build_jmp() function for details */

	/* check for long jumps and insert if necessary, we also verify that
	 * all our jump targets are valid at this point in the process */
	b_iter = b_tail;
	do {
		res_cnt = 0;
		for (iter = b_iter->blk_cnt - 1; iter >= 0; iter--) {
			i_iter = &b_iter->blks[iter];
			switch (i_iter->jt.type) {
			case TGT_NONE:
			case TGT_IMM:
				break;
			case TGT_PTR_HSH:
				h_val = i_iter->jt.tgt.hash;
				rc = _gen_bpf_build_jmp(state, b_tail,
							b_iter, iter,
							h_val);
				if (rc < 0)
					goto build_bpf_free_blks;
				res_cnt += rc;
				break;
			default:
				/* fatal error */
				goto build_bpf_free_blks;
			}
			switch (i_iter->jf.type) {
			case TGT_NONE:
			case TGT_IMM:
				break;
			case TGT_PTR_HSH:
				h_val = i_iter->jf.tgt.hash;
				rc = _gen_bpf_build_jmp(state, b_tail,
							b_iter, iter,
							h_val);
				if (rc < 0)
					goto build_bpf_free_blks;
				res_cnt += rc;
				break;
			default:
				/* fatal error */
				goto build_bpf_free_blks;
			}
		}
		if (res_cnt == 0)
			b_iter = b_iter->prev;
	} while (b_iter != NULL);

	/* build the bpf program */
	do {
		b_iter = b_head;
		/* resolve the TGT_PTR_HSH jumps */
		for (iter = 0; iter < b_iter->blk_cnt; iter++) {
			i_iter = &b_iter->blks[iter];
			if (i_iter->jt.type == TGT_PTR_HSH) {
				h_val = i_iter->jt.tgt.hash;
				jmp_len = b_iter->blk_cnt - (iter + 1);
				b_jmp = b_iter->next;
				while (b_jmp != NULL && b_jmp->hash != h_val) {
					jmp_len += b_jmp->blk_cnt;
					b_jmp = b_jmp->next;
				}
				if (b_jmp == NULL || jmp_len > _BPF_JMP_MAX)
					goto build_bpf_free_blks;
				i_iter->jt = _BPF_JMP_IMM(jmp_len);
			}
			if (i_iter->jf.type == TGT_PTR_HSH) {
				h_val = i_iter->jf.tgt.hash;
				jmp_len = b_iter->blk_cnt - (iter + 1);
				b_jmp = b_iter->next;
				while (b_jmp != NULL && b_jmp->hash != h_val) {
					jmp_len += b_jmp->blk_cnt;
					b_jmp = b_jmp->next;
				}
				if (b_jmp == NULL || jmp_len > _BPF_JMP_MAX)
					goto build_bpf_free_blks;
				i_iter->jf = _BPF_JMP_IMM(jmp_len);
			}
			if (i_iter->k.type == TGT_PTR_HSH) {
				h_val = i_iter->k.tgt.hash;
				jmp_len = b_iter->blk_cnt - (iter + 1);
				b_jmp = b_tail;
				while (b_jmp->hash != h_val)
					b_jmp = b_jmp->prev;
				b_jmp = b_jmp->prev;
				while (b_jmp != b_iter) {
					jmp_len += b_jmp->blk_cnt;
					b_jmp = b_jmp->prev;
				}
				if (b_jmp == NULL)
					goto build_bpf_free_blks;
				i_iter->k = _BPF_K(state->arch, jmp_len);
			}
		}

		/* build the bpf program */
		if (_bpf_append_blk(state->bpf, b_iter) < 0)
			goto build_bpf_free_blks;

		/* we're done with the block, free it */
		b_head = b_iter->next;
		_blk_free(state, b_iter);
	} while (b_head != NULL);

	return 0;

build_bpf_free_blks:
	b_iter = b_head;
	while (b_iter != NULL) {
		b_jmp = b_iter->next;
		_hsh_remove(state, b_iter->hash);
		__blk_free(state, b_iter);
		b_iter = b_jmp;
	}
	return -EFAULT;
}

/**
 * Generate a BPF representation of the filter DB
 * @param col the seccomp filter collection
 *
 * This function generates a BPF representation of the given filter collection.
 * Returns a pointer to a valid bpf_program on success, NULL on failure.
 *
 */
struct bpf_program *gen_bpf_generate(const struct db_filter_col *col)
{
	int rc;
	struct bpf_state state;

	memset(&state, 0, sizeof(state));
	state.attr = &col->attr;

	state.bpf = malloc(sizeof(*(state.bpf)));
	if (state.bpf == NULL)
		return NULL;
	memset(state.bpf, 0, sizeof(*(state.bpf)));

	rc = _gen_bpf_build_bpf(&state, col);
	if (rc < 0)
		goto bpf_generate_end;

bpf_generate_end:
	if (rc < 0)
		_state_release(&state);
	return state.bpf;
}

/**
 * Free memory associated with a BPF representation
 * @param fprog the BPF representation
 *
 * Free the memory associated with a BPF representation generated by the
 * gen_bpf_generate() function.
 *
 */
void gen_bpf_release(struct bpf_program *program)
{
	_program_free(program);
}
