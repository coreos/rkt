/**
 * Seccomp Library test program
 *
 * Copyright (c) 2013 Red Hat <pmoore@redhat.com>
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
#include <fcntl.h>
#include <string.h>
#include <unistd.h>
#include <sys/types.h>
#include <sys/stat.h>

#include <seccomp.h>

#include "util.h"

int main(int argc, char *argv[])
{
	int rc;
	int fd;
	scmp_filter_ctx ctx = NULL;
	const char buf[] = "testing";
	ssize_t buf_len = strlen(buf);

	rc = util_action_parse(argv[1]);
	if (rc != SCMP_ACT_ALLOW) {
		rc = 1;
		goto out;
	}

	rc = util_trap_install();
	if (rc != 0)
		goto out;

	fd = open("/dev/null", O_WRONLY | O_CREAT, S_IRUSR | S_IWUSR);
	if (fd < 0) {
		rc = errno;
		goto out;
	}

	ctx = seccomp_init(SCMP_ACT_TRAP);
	if (ctx == NULL)
		return ENOMEM;

	rc = seccomp_rule_add(ctx, SCMP_ACT_ALLOW, SCMP_SYS(write), 1,
			      SCMP_A0(SCMP_CMP_EQ, fd));
	if (rc != 0)
		goto out;
	rc = seccomp_rule_add(ctx, SCMP_ACT_ALLOW, SCMP_SYS(close), 0);
	if (rc != 0)
		goto out;
	rc = seccomp_rule_add(ctx, SCMP_ACT_ALLOW, SCMP_SYS(rt_sigreturn), 0);
	if (rc != 0)
		goto out;
	rc = seccomp_rule_add(ctx, SCMP_ACT_ALLOW, SCMP_SYS(exit_group), 0);
	if (rc != 0)
		goto out;

	rc = seccomp_load(ctx);
	if (rc != 0)
		goto out;

	if (write(fd, buf, buf_len) < buf_len) {
		rc = errno;
		goto out;
	}
	if (close(fd) < 0) {
		rc = errno;
		goto out;
	}

	rc = 160;

out:
	seccomp_release(ctx);
	return (rc < 0 ? -rc : rc);
}
