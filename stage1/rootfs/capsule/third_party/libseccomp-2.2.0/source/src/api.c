/**
 * Seccomp Library API
 *
 * Copyright (c) 2012,2013 Red Hat <pmoore@redhat.com>
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

#include <endian.h>
#include <errno.h>
#include <inttypes.h>
#include <unistd.h>
#include <stdarg.h>
#include <stdlib.h>
#include <string.h>
#include <stdbool.h>

#include <seccomp.h>

#include "arch.h"
#include "db.h"
#include "gen_pfc.h"
#include "gen_bpf.h"
#include "system.h"

#define API	__attribute__((visibility("default")))

/**
 * Validate a filter context
 * @param ctx the filter context
 *
 * Attempt to validate the provided filter context.  Returns zero if the
 * context is valid, negative values on failure.
 *
 */
static int _ctx_valid(const scmp_filter_ctx *ctx)
{
	return db_col_valid((struct db_filter_col *)ctx);
}

/**
 * Validate a syscall number
 * @param syscall the syscall number
 *
 * Attempt to perform basic syscall number validation.  Returns zero of the
 * syscall appears valid, negative values on failure.
 *
 */
static int _syscall_valid(int syscall)
{
	if (syscall <= -1 && syscall >= -99)
		return -EINVAL;
	return 0;
}

/* NOTE - function header comment in include/seccomp.h */
API scmp_filter_ctx seccomp_init(uint32_t def_action)
{
	struct db_filter_col *col;
	struct db_filter *db;

	if (db_action_valid(def_action) < 0)
		return NULL;

	col = db_col_init(def_action);
	if (col == NULL)
		return NULL;
	db = db_init(arch_def_native);
	if (db == NULL)
		goto init_failure_col;

	if (db_col_db_add(col, db) < 0)
		goto init_failure_db;

	return col;

init_failure_db:
	db_release(db);
init_failure_col:
	db_col_release(col);
	return NULL;
}

/* NOTE - function header comment in include/seccomp.h */
API int seccomp_reset(scmp_filter_ctx ctx, uint32_t def_action)
{
	int rc;
	struct db_filter_col *col = (struct db_filter_col *)ctx;
	struct db_filter *db;

	if (ctx == NULL || db_action_valid(def_action) < 0)
		return -EINVAL;

	db_col_reset(col, def_action);

	db = db_init(arch_def_native);
	if (db == NULL)
		return -ENOMEM;
	rc = db_col_db_add(col, db);
	if (rc < 0)
		db_release(db);

	return rc;
}

/* NOTE - function header comment in include/seccomp.h */
API void seccomp_release(scmp_filter_ctx ctx)
{
	db_col_release((struct db_filter_col *)ctx);
}

/* NOTE - function header comment in include/seccomp.h */
API int seccomp_merge(scmp_filter_ctx ctx_dst,
		      scmp_filter_ctx ctx_src)
{
	struct db_filter_col *col_dst = (struct db_filter_col *)ctx_dst;
	struct db_filter_col *col_src = (struct db_filter_col *)ctx_src;

	if (db_col_valid(col_dst) || db_col_valid(col_src))
		return -EINVAL;

	/* NOTE: only the default action, NNP, and TSYNC settings must match */
	if ((col_dst->attr.act_default != col_src->attr.act_default) ||
	    (col_dst->attr.nnp_enable != col_src->attr.nnp_enable) ||
	    (col_dst->attr.tsync_enable != col_src->attr.tsync_enable))
		return -EINVAL;

	return db_col_merge(col_dst, col_src);
}

/* NOTE - function header comment in include/seccomp.h */
API uint32_t seccomp_arch_resolve_name(const char *arch_name)
{
	const struct arch_def *arch;

	if (arch_name == NULL)
		return 0;

	arch = arch_def_lookup_name(arch_name);
	if (arch == NULL)
		return 0;

	return arch->token;
}

/* NOTE - function header comment in include/seccomp.h */
API uint32_t seccomp_arch_native(void)
{
	return arch_def_native->token;
}

/* NOTE - function header comment in include/seccomp.h */
API int seccomp_arch_exist(const scmp_filter_ctx ctx,
			   uint32_t arch_token)
{
	struct db_filter_col *col = (struct db_filter_col *)ctx;

	if (arch_token == 0)
		arch_token = arch_def_native->token;

	if (arch_valid(arch_token))
		return -EINVAL;

	return (db_col_arch_exist(col, arch_token) ? 0 : -EEXIST);
}

/* NOTE - function header comment in include/seccomp.h */
API int seccomp_arch_add(scmp_filter_ctx ctx, uint32_t arch_token)
{
	int rc;
	const struct arch_def *arch;
	struct db_filter *db;
	struct db_filter_col *col = (struct db_filter_col *)ctx;

	if (arch_token == 0)
		arch_token = arch_def_native->token;

	if (arch_valid(arch_token))
		return -EINVAL;
	if (db_col_arch_exist(col, arch_token))
		return -EEXIST;

	arch = arch_def_lookup(arch_token);
	if (arch == NULL)
		return -EFAULT;
	db = db_init(arch);
	if (db == NULL)
		return -ENOMEM;
	rc = db_col_db_add(col, db);
	if (rc < 0)
		db_release(db);

	return rc;
}

/* NOTE - function header comment in include/seccomp.h */
API int seccomp_arch_remove(scmp_filter_ctx ctx, uint32_t arch_token)
{
	struct db_filter_col *col = (struct db_filter_col *)ctx;

	if (arch_token == 0)
		arch_token = arch_def_native->token;

	if (arch_valid(arch_token))
		return -EINVAL;
	if (db_col_arch_exist(col, arch_token) != -EEXIST)
		return -EEXIST;

	return db_col_db_remove(col, arch_token);
}

/* NOTE - function header comment in include/seccomp.h */
API int seccomp_load(const scmp_filter_ctx ctx)
{
	struct db_filter_col *col;

	if (_ctx_valid(ctx))
		return -EINVAL;
	col = (struct db_filter_col *)ctx;

	return sys_filter_load(col);
}

/* NOTE - function header comment in include/seccomp.h */
API int seccomp_attr_get(const scmp_filter_ctx ctx,
			 enum scmp_filter_attr attr, uint32_t *value)
{
	if (_ctx_valid(ctx))
		return -EINVAL;

	return db_col_attr_get((const struct db_filter_col *)ctx, attr, value);
}

/* NOTE - function header comment in include/seccomp.h */
API int seccomp_attr_set(scmp_filter_ctx ctx,
			 enum scmp_filter_attr attr, uint32_t value)
{
	if (_ctx_valid(ctx))
		return -EINVAL;

	return db_col_attr_set((struct db_filter_col *)ctx, attr, value);
}

/* NOTE - function header comment in include/seccomp.h */
API char *seccomp_syscall_resolve_num_arch(uint32_t arch_token, int num)
{
	const struct arch_def *arch;
	const char *name;

	if (arch_token == 0)
		arch_token = arch_def_native->token;
	if (arch_valid(arch_token))
		return NULL;
	arch = arch_def_lookup(arch_token);
	if (arch == NULL)
		return NULL;

	name = arch_syscall_resolve_num(arch, num);
	if (name == NULL)
		return NULL;

	return strdup(name);
}

/* NOTE - function header comment in include/seccomp.h */
API int seccomp_syscall_resolve_name_arch(uint32_t arch_token, const char *name)
{
	const struct arch_def *arch;

	if (name == NULL)
		return __NR_SCMP_ERROR;

	if (arch_token == 0)
		arch_token = arch_def_native->token;
	if (arch_valid(arch_token))
		return __NR_SCMP_ERROR;
	arch = arch_def_lookup(arch_token);
	if (arch == NULL)
		return __NR_SCMP_ERROR;

	return arch_syscall_resolve_name(arch, name);
}

/* NOTE - function header comment in include/seccomp.h */
API int seccomp_syscall_resolve_name_rewrite(uint32_t arch_token,
					     const char *name)
{
	int syscall;
	const struct arch_def *arch;

	if (name == NULL)
		return __NR_SCMP_ERROR;

	if (arch_token == 0)
		arch_token = arch_def_native->token;
	if (arch_valid(arch_token))
		return __NR_SCMP_ERROR;
	arch = arch_def_lookup(arch_token);
	if (arch == NULL)
		return __NR_SCMP_ERROR;

	syscall = arch_syscall_resolve_name(arch, name);
	if (syscall == __NR_SCMP_ERROR)
		return __NR_SCMP_ERROR;
	if (arch_syscall_rewrite(arch, 0, &syscall) < 0)
		return __NR_SCMP_ERROR;

	return syscall;
}

/* NOTE - function header comment in include/seccomp.h */
API int seccomp_syscall_resolve_name(const char *name)
{
	return seccomp_syscall_resolve_name_arch(SCMP_ARCH_NATIVE, name);
}

/* NOTE - function header comment in include/seccomp.h */
API int seccomp_syscall_priority(scmp_filter_ctx ctx,
				 int syscall, uint8_t priority)
{
	int rc = 0, rc_tmp;
	unsigned int iter;
	int sc_tmp;
	struct db_filter_col *col;
	struct db_filter *filter;

	if (_ctx_valid(ctx) || _syscall_valid(syscall))
		return -EINVAL;
	col = (struct db_filter_col *)ctx;

	for (iter = 0; iter < col->filter_cnt; iter++) {
		filter = col->filters[iter];
		sc_tmp = syscall;

		rc_tmp = arch_syscall_translate(filter->arch, &sc_tmp);
		if (rc_tmp < 0)
			goto syscall_priority_failure;

		/* if this is a pseudo syscall (syscall < 0) then we need to
		 * rewrite the syscall for some arch specific reason */
		if (sc_tmp < 0) {
			/* we set this as a strict op - we don't really care
			 * since priorities are a "best effort" thing - as we
			 * want to catch the -EDOM error and bail on this
			 * architecture */
			rc_tmp = arch_syscall_rewrite(filter->arch, 1, &sc_tmp);
			if (rc_tmp == -EDOM)
				continue;
			if (rc_tmp < 0)
				goto syscall_priority_failure;
		}

		rc_tmp = db_syscall_priority(filter, sc_tmp, priority);

syscall_priority_failure:
		if (rc == 0 && rc_tmp < 0)
			rc = rc_tmp;
	}

	return rc;
}

/**
 * Add a new rule to the current filter
 * @param col the filter collection
 * @param strict the strict flag
 * @param action the filter action
 * @param syscall the syscall number
 * @param arg_cnt the number of argument filters in the argument filter chain
 * @param arg_list the argument filter chain, (uint, enum scmp_compare, ulong)
 *
 * This function adds a new argument/comparison/value to the seccomp filter for
 * a syscall; multiple arguments can be specified and they will be chained
 * together (essentially AND'd together) in the filter.  When the strict flag
 * is true the function will fail if the exact rule can not be added to the
 * filter, if the strict flag is false the function will not fail if the
 * function needs to adjust the rule due to architecture specifics.  Returns
 * zero on success, negative values on failure.
 *
 */
static int _seccomp_rule_add(struct db_filter_col *col,
			     bool strict, uint32_t action, int syscall,
			     unsigned int arg_cnt,
			     const struct scmp_arg_cmp *arg_array)
{
	int rc = 0, rc_tmp;
	int sc_tmp;
	unsigned int iter;
	unsigned int chain_len;
	unsigned int arg_num;
	size_t chain_size;
	struct db_filter *filter;
	struct db_api_arg *chain = NULL, *chain_tmp;
	struct scmp_arg_cmp arg_data;

	if (arg_cnt > 0 && arg_array == NULL)
		return -EINVAL;

	if (db_col_valid(col) || _syscall_valid(syscall))
		return -EINVAL;

	rc = db_action_valid(action);
	if (rc < 0)
		return rc;
	if (action == col->attr.act_default)
		return -EPERM;

	if (strict && col->filter_cnt > 1)
		return -EOPNOTSUPP;

	/* collect the arguments for the filter rule */
	chain_len = ARG_COUNT_MAX;
	chain_size = sizeof(*chain) * chain_len;
	chain = malloc(chain_size);
	if (chain == NULL)
		return -ENOMEM;
	memset(chain, 0, chain_size);
	for (iter = 0; iter < arg_cnt; iter++) {
		arg_data = arg_array[iter];
		arg_num = arg_data.arg;
		if (arg_num < chain_len && chain[arg_num].valid == 0) {
			chain[arg_num].valid = 1;
			chain[arg_num].arg = arg_num;
			chain[arg_num].op = arg_data.op;
			/* XXX - we should check datum/mask size against the
			 *	 arch definition, e.g. 64 bit datum on x86 */
			switch (chain[arg_num].op) {
			case SCMP_CMP_NE:
			case SCMP_CMP_LT:
			case SCMP_CMP_LE:
			case SCMP_CMP_EQ:
			case SCMP_CMP_GE:
			case SCMP_CMP_GT:
				chain[arg_num].mask = DATUM_MAX;
				chain[arg_num].datum = arg_data.datum_a;
				break;
			case SCMP_CMP_MASKED_EQ:
				chain[arg_num].mask = arg_data.datum_a;
				chain[arg_num].datum = arg_data.datum_b;
				break;
			default:
				rc = -EINVAL;
				goto rule_add_return;
			}
		} else {
			rc = -EINVAL;
			goto rule_add_return;
		}
	}

	for (iter = 0; iter < col->filter_cnt; iter++) {
		filter = col->filters[iter];
		sc_tmp = syscall;

		rc_tmp = arch_syscall_translate(filter->arch, &sc_tmp);
		if (rc_tmp < 0)
			goto rule_add_failure;

		/* if this is a pseudo syscall (syscall < 0) then we need to
		 * rewrite the rule for some arch specific reason */
		if (sc_tmp < 0) {
			/* make a private copy of the chain */
			chain_tmp = malloc(chain_size);
			if (chain_tmp == NULL) {
				rc = -ENOMEM;
				goto rule_add_failure;
			}
			memcpy(chain_tmp, chain, chain_size);

			/* mangle the private chain copy */
			rc_tmp = arch_filter_rewrite(filter->arch, strict,
						     &sc_tmp, chain_tmp);
			if ((rc == -EDOM) && (!strict)) {
				free(chain_tmp);
				continue;
			}
			if (rc_tmp < 0) {
				free(chain_tmp);
				goto rule_add_failure;
			}

			/* add the new rule to the existing filter */
			rc_tmp = db_rule_add(filter, action, sc_tmp, chain_tmp);
			free(chain_tmp);
		} else
			/* add the new rule to the existing filter */
			rc_tmp = db_rule_add(filter, action, sc_tmp, chain);

rule_add_failure:
		if (rc == 0 && rc_tmp < 0)
			rc = rc_tmp;
	}

rule_add_return:
	if (chain != NULL)
		free(chain);
	return rc;
}

/* NOTE - function header comment in include/seccomp.h */
API int seccomp_rule_add_array(scmp_filter_ctx ctx,
			       uint32_t action, int syscall,
			       unsigned int arg_cnt,
			       const struct scmp_arg_cmp *arg_array)
{
	/* arg_cnt is unsigned, so no need to check the lower bound */
	if (arg_cnt > ARG_COUNT_MAX)
		return -EINVAL;

	return _seccomp_rule_add((struct db_filter_col *)ctx,
				 0, action, syscall, arg_cnt, arg_array);
}

/* NOTE - function header comment in include/seccomp.h */
API int seccomp_rule_add(scmp_filter_ctx ctx,
			 uint32_t action, int syscall,
			 unsigned int arg_cnt, ...)
{
	int rc;
	int iter;
	struct scmp_arg_cmp arg_array[ARG_COUNT_MAX];
	va_list arg_list;

	/* arg_cnt is unsigned, so no need to check the lower bound */
	if (arg_cnt > ARG_COUNT_MAX)
		return -EINVAL;

	va_start(arg_list, arg_cnt);
	for (iter = 0; iter < arg_cnt; ++iter)
		arg_array[iter] = va_arg(arg_list, struct scmp_arg_cmp);
	rc = seccomp_rule_add_array(ctx, action, syscall, arg_cnt, arg_array);
	va_end(arg_list);

	return rc;
}

/* NOTE - function header comment in include/seccomp.h */
API int seccomp_rule_add_exact_array(scmp_filter_ctx ctx,
				     uint32_t action, int syscall,
				     unsigned int arg_cnt,
				     const struct scmp_arg_cmp *arg_array)
{
	/* arg_cnt is unsigned, so no need to check the lower bound */
	if (arg_cnt > ARG_COUNT_MAX)
		return -EINVAL;

	return _seccomp_rule_add((struct db_filter_col *)ctx,
				 1, action, syscall, arg_cnt, arg_array);
}

/* NOTE - function header comment in include/seccomp.h */
API int seccomp_rule_add_exact(scmp_filter_ctx ctx,
			       uint32_t action, int syscall,
			       unsigned int arg_cnt, ...)
{
	int rc;
	int iter;
	struct scmp_arg_cmp arg_array[ARG_COUNT_MAX];
	va_list arg_list;

	/* arg_cnt is unsigned, so no need to check the lower bound */
	if (arg_cnt > ARG_COUNT_MAX)
		return -EINVAL;

	va_start(arg_list, arg_cnt);
	for (iter = 0; iter < arg_cnt; ++iter)
		arg_array[iter] = va_arg(arg_list, struct scmp_arg_cmp);
	rc = seccomp_rule_add_exact_array(ctx,
					  action, syscall, arg_cnt, arg_array);
	va_end(arg_list);

	return rc;
}

/* NOTE - function header comment in include/seccomp.h */
API int seccomp_export_pfc(const scmp_filter_ctx ctx, int fd)
{
	if (_ctx_valid(ctx))
		return -EINVAL;

	return gen_pfc_generate((struct db_filter_col *)ctx, fd);
}

/* NOTE - function header comment in include/seccomp.h */
API int seccomp_export_bpf(const scmp_filter_ctx ctx, int fd)
{
	int rc;
	struct bpf_program *program;

	if (_ctx_valid(ctx))
		return -EINVAL;

	program = gen_bpf_generate((struct db_filter_col *)ctx);
	if (program == NULL)
		return -ENOMEM;
	rc = write(fd, program->blks, BPF_PGM_SIZE(program));
	gen_bpf_release(program);
	if (rc < 0)
		return -errno;

	return 0;
}
