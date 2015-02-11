/**
 * Seccomp Library test program
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
#include <unistd.h>

#include <seccomp.h>

#include "util.h"

int main(int argc, char *argv[])
{
	int rc;
	struct util_options opts;
	scmp_filter_ctx ctx_64 = NULL, ctx_32 = NULL;

	rc = util_getopt(argc, argv, &opts);
	if (rc < 0)
		goto out_all;

	ctx_32 = seccomp_init(SCMP_ACT_KILL);
	if (ctx_32 == NULL) {
		rc = -ENOMEM;
		goto out_all;
	}
	ctx_64 = seccomp_init(SCMP_ACT_KILL);
	if (ctx_64 == NULL) {
		rc = -ENOMEM;
		goto out_all;
	}

	rc = seccomp_arch_remove(ctx_32, SCMP_ARCH_NATIVE);
	if (rc != 0)
		goto out;
	rc = seccomp_arch_remove(ctx_64, SCMP_ARCH_NATIVE);
	if (rc != 0)
		goto out;

	rc = seccomp_arch_add(ctx_32, SCMP_ARCH_X86);
	if (rc != 0)
		goto out_all;
	rc = seccomp_arch_add(ctx_64, SCMP_ARCH_X86_64);
	if (rc != 0)
		goto out_all;

	rc = seccomp_rule_add(ctx_32, SCMP_ACT_ALLOW, SCMP_SYS(read), 1,
			      SCMP_A0(SCMP_CMP_EQ, STDIN_FILENO));
	if (rc != 0)
		goto out_all;

	rc = seccomp_rule_add(ctx_32, SCMP_ACT_ALLOW, SCMP_SYS(write), 1,
			      SCMP_A0(SCMP_CMP_EQ, STDOUT_FILENO));
	if (rc != 0)
		goto out_all;

	rc = seccomp_rule_add(ctx_32, SCMP_ACT_ALLOW, SCMP_SYS(write), 1,
			      SCMP_A0(SCMP_CMP_EQ, STDERR_FILENO));
	if (rc != 0)
		goto out_all;

	rc = seccomp_rule_add(ctx_32, SCMP_ACT_ALLOW, SCMP_SYS(close), 0);
	if (rc != 0)
		goto out_all;

	rc = seccomp_rule_add(ctx_64, SCMP_ACT_ALLOW, SCMP_SYS(socket), 0);
	if (rc != 0)
		goto out_all;

	rc = seccomp_rule_add(ctx_64, SCMP_ACT_ALLOW, SCMP_SYS(connect), 0);
	if (rc != 0)
		goto out_all;

	rc = seccomp_rule_add(ctx_64, SCMP_ACT_ALLOW, SCMP_SYS(shutdown), 0);
	if (rc != 0)
		goto out_all;

	rc = seccomp_merge(ctx_64, ctx_32);
	if (rc != 0)
		goto out_all;

	/* NOTE: ctx_32 is no longer valid at this point */

	rc = util_filter_output(&opts, ctx_64);
	if (rc)
		goto out;

out:
	seccomp_release(ctx_64);
	return (rc < 0 ? -rc : rc);
out_all:
	seccomp_release(ctx_32);
	goto out;
}
