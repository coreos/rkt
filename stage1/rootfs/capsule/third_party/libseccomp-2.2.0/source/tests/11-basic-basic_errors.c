/**
 * Seccomp Library test program
 *
 * Copyright IBM Corp. 2012
 * Author: Corey Bryant <coreyb@linux.vnet.ibm.com>
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

int main(int argc, char *argv[])
{
	int rc;
	scmp_filter_ctx ctx;

	/* seccomp_init errors */
	ctx = seccomp_init(SCMP_ACT_ALLOW + 1);
	if (ctx != NULL)
		return -1;

	ctx = seccomp_init(SCMP_ACT_ALLOW);
	if (ctx == NULL)
		return -1;
	seccomp_release(ctx);
	ctx = NULL;

	/* seccomp_reset error */
	rc = seccomp_reset(ctx, SCMP_ACT_KILL + 1);
	if (rc != -EINVAL)
		return -1;
	rc = seccomp_reset(ctx, SCMP_ACT_KILL);
	if (rc != -EINVAL)
		return -1;

	/* seccomp_load error */
	rc = seccomp_load(ctx);
	if (rc != -EINVAL)
		return -1;

	/* seccomp_syscall_priority errors */
	rc = seccomp_syscall_priority(ctx, SCMP_SYS(read), 1);
	if (rc != -EINVAL)
		return -1;

	ctx = seccomp_init(SCMP_ACT_ALLOW);
	if (ctx == NULL)
		return -1;
	else {
		rc = seccomp_syscall_priority(ctx, -10, 1);
		if (rc != -EINVAL)
			return -1;
	}
	seccomp_release(ctx);
	ctx = NULL;

	/* seccomp_rule_add errors */
	rc = seccomp_rule_add(ctx, SCMP_ACT_ALLOW, SCMP_SYS(read), 1,
			      SCMP_A0(SCMP_CMP_EQ, 0));
	if (rc != -EINVAL)
		return -1;

	ctx = seccomp_init(SCMP_ACT_ALLOW);
	if (ctx == NULL)
		return -1;
	else {
		rc = seccomp_rule_add(ctx, SCMP_ACT_ALLOW, SCMP_SYS(read), 0);
		if (rc != -EPERM)
			return -1;
		rc = seccomp_rule_add(ctx, SCMP_ACT_KILL - 1, SCMP_SYS(read), 0);
		if (rc != -EINVAL)
			return -1;
		rc = seccomp_rule_add(ctx, SCMP_ACT_KILL, SCMP_SYS(read), 7);
		if (rc != -EINVAL)
			return -1;
		rc = seccomp_rule_add(ctx, SCMP_ACT_KILL, SCMP_SYS(read), 7,
				      SCMP_A0(SCMP_CMP_EQ, 0),
				      SCMP_A1(SCMP_CMP_EQ, 0),
				      SCMP_A2(SCMP_CMP_EQ, 0),
				      SCMP_A3(SCMP_CMP_EQ, 0),
				      SCMP_A4(SCMP_CMP_EQ, 0),
				      SCMP_A5(SCMP_CMP_EQ, 0),
				      SCMP_CMP(6, SCMP_CMP_EQ, 0));
		if (rc != -EINVAL)
			return -1;
		rc = seccomp_rule_add(ctx, SCMP_ACT_KILL, SCMP_SYS(read), 1,
				      SCMP_A0(_SCMP_CMP_MIN, 0));
		if (rc != -EINVAL)
			return -1;
		rc = seccomp_rule_add(ctx, SCMP_ACT_KILL, SCMP_SYS(read), 1,
				      SCMP_A0(_SCMP_CMP_MAX, 0));
		if (rc != -EINVAL)
			return -1;
		rc = seccomp_rule_add_exact(ctx, SCMP_ACT_KILL, -10001, 0);
		if (rc != -EDOM)
			return -1;
	}
	seccomp_release(ctx);
	ctx = NULL;

	/* seccomp_rule_add_exact error */
	ctx = seccomp_init(SCMP_ACT_ALLOW);
	if (ctx == NULL)
		return -1;
	rc = seccomp_arch_remove(ctx, SCMP_ARCH_NATIVE);
	if (rc != 0)
		return -1;
	rc = seccomp_arch_add(ctx, SCMP_ARCH_X86);
	if (rc != 0)
		return -1;
	rc = seccomp_rule_add_exact(ctx, SCMP_ACT_KILL, SCMP_SYS(socket), 1,
				    SCMP_A0(SCMP_CMP_EQ, 2));
	if (rc != -EINVAL)
		return -1;
	seccomp_release(ctx);
	ctx = NULL;

	/* seccomp_export_pfc errors */
	rc = seccomp_export_pfc(ctx, STDOUT_FILENO);
	if (rc != -EINVAL)
		return -1;

	ctx = seccomp_init(SCMP_ACT_ALLOW);
	if (ctx == NULL)
		return -1;
	else {
		rc = seccomp_export_pfc(ctx, sysconf(_SC_OPEN_MAX) - 1);
		if (rc != EBADF)
			return -1;
	}
	seccomp_release(ctx);
	ctx = NULL;

	/* seccomp_export_bpf errors */
	rc = seccomp_export_bpf(ctx, STDOUT_FILENO);
	if (rc != -EINVAL)
		return -1;

	ctx = seccomp_init(SCMP_ACT_ALLOW);
	if (ctx == NULL)
		return -1;
	else {
		rc = seccomp_export_bpf(ctx, sysconf(_SC_OPEN_MAX) - 1);
		if (rc != -EBADF)
			return -1;
	}
	seccomp_release(ctx);
	ctx = NULL;

	return 0;
}
