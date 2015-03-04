/**
 * Enhanced Seccomp Filter DB
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

#ifndef _FILTER_DB_H
#define _FILTER_DB_H

#include <inttypes.h>
#include <stdbool.h>

#include <seccomp.h>

#include "arch.h"

/* XXX - need to provide doxygen comments for the types here */

struct db_api_arg {
	unsigned int arg;
	unsigned int op;
	scmp_datum_t mask;
	scmp_datum_t datum;

	bool valid;
};

struct db_arg_chain_tree {
	/* argument number (a0 = 0, a1 = 1, etc.) */
	unsigned int arg;
	/* argument bpf offset */
	unsigned int arg_offset;

	/* comparison operator */
	enum scmp_compare op;
	/* syscall argument value */
	uint32_t mask;
	uint32_t datum;

	/* actions */
	bool act_t_flg;
	bool act_f_flg;
	uint32_t act_t;
	uint32_t act_f;

	/* list of nodes on this level */
	struct db_arg_chain_tree *lvl_prv, *lvl_nxt;

	/* next node in the chain */
	struct db_arg_chain_tree *nxt_t;
	struct db_arg_chain_tree *nxt_f;

	unsigned int refcnt;
};
#define ARG_MASK_MAX		((uint32_t)-1)
#define db_chain_lt(x,y) \
	(((x)->arg < (y)->arg) || \
	 (((x)->arg == (y)->arg) && \
	  (((x)->op < (y)->op) || (((x)->mask & (y)->mask) == (y)->mask))))
#define db_chain_eq(x,y) \
	(((x)->arg == (y)->arg) && \
	 ((x)->op == (y)->op) && ((x)->datum == (y)->datum) && \
	 ((x)->mask == (y)->mask))
#define db_chain_gt(x,y) \
	(((x)->arg > (y)->arg) || \
	 (((x)->arg == (y)->arg) && \
	  (((x)->op > (y)->op) || (((x)->mask & (y)->mask) != (y)->mask))))
#define db_chain_action(x) \
	(((x)->act_t_flg) || ((x)->act_f_flg))
#define db_chain_zombie(x) \
	((x)->nxt_t == NULL && !((x)->act_t_flg) && \
	 (x)->nxt_f == NULL && !((x)->act_f_flg))
#define db_chain_leaf(x) \
	((x)->nxt_t == NULL && (x)->nxt_f == NULL)
#define db_chain_eq_result(x,y) \
	((((x)->nxt_t != NULL && (y)->nxt_t != NULL) || \
	  ((x)->nxt_t == NULL && (y)->nxt_t == NULL)) && \
	 (((x)->nxt_f != NULL && (y)->nxt_f != NULL) || \
	  ((x)->nxt_f == NULL && (y)->nxt_f == NULL)) && \
	 ((x)->act_t_flg == (y)->act_t_flg) && \
	 ((x)->act_f_flg == (y)->act_f_flg) && \
	 (((x)->act_t_flg && (x)->act_t == (y)->act_t) || \
	  (!((x)->act_t_flg))) && \
	 (((x)->act_f_flg && (x)->act_f == (y)->act_f) || \
	  (!((x)->act_f_flg))))

struct db_sys_list {
	/* native syscall number */
	unsigned int num;

	/* priority - higher is better */
	unsigned int priority;

	/* the argument chain heads */
	struct db_arg_chain_tree *chains;
	unsigned int node_cnt;

	/* action in the case of no argument chains */
	uint32_t action;

	struct db_sys_list *next;
	/* temporary use only by the BPF generator */
	struct db_sys_list *pri_prv, *pri_nxt;

	bool valid;
};

struct db_filter_attr {
	/* action to take if we don't match an explicit allow/deny */
	uint32_t act_default;
	/* action to take if we don't match the architecture */
	uint32_t act_badarch;
	/* NO_NEW_PRIVS related attributes */
	uint32_t nnp_enable;
	/* SECCOMP_FILTER_FLAG_TSYNC related attributes */
	uint32_t tsync_enable;
};

struct db_filter {
	/* target architecture */
	const struct arch_def *arch;

	/* syscall filters, kept as a sorted single-linked list */
	struct db_sys_list *syscalls;
};

struct db_filter_col {
	/* verification / state */
	int state;

	/* attributes */
	struct db_filter_attr attr;

	/* individual filters */
	int endian;
	struct db_filter **filters;
	unsigned int filter_cnt;
};

/**
 * Iterate over each item in the DB list
 * @param iter the iterator
 * @param list the list
 *
 * This macro acts as for()/while() conditional and iterates the following
 * statement for each item in the given list.
 *
 */
#define db_list_foreach(iter,list) \
	for (iter = (list); iter != NULL; iter = iter->next)

int db_action_valid(uint32_t action);

struct db_filter_col *db_col_init(uint32_t def_action);
void db_col_reset(struct db_filter_col *col, uint32_t def_action);
void db_col_release(struct db_filter_col *col);

int db_col_valid(struct db_filter_col *col);

int db_col_merge(struct db_filter_col *col_dst, struct db_filter_col *col_src);

int db_col_arch_exist(struct db_filter_col *col, uint32_t arch_token);

int db_col_attr_get(const struct db_filter_col *col,
		    enum scmp_filter_attr attr, uint32_t *value);
int db_col_attr_set(struct db_filter_col *col,
		    enum scmp_filter_attr attr, uint32_t value);

int db_col_db_add(struct db_filter_col *col, struct db_filter *db);
int db_col_db_remove(struct db_filter_col *col, uint32_t arch_token);

struct db_filter *db_init(const struct arch_def *arch);
void db_reset(struct db_filter *db);
void db_release(struct db_filter *db);

int db_syscall_priority(struct db_filter *db,
			unsigned int syscall, uint8_t priority);

int db_rule_add(struct db_filter *db, uint32_t action, unsigned int syscall,
		struct db_api_arg *chain);

#endif
