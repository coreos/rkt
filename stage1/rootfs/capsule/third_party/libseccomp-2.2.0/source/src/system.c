/**
 * Seccomp System Interfaces
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

#include <stdlib.h>
#include <errno.h>
#include <sys/prctl.h>

#include <seccomp.h>

#include "db.h"
#include "gen_bpf.h"
#include "system.h"

/**
 * Check to see if a seccomp() flag is supported
 * @param flag the seccomp() flag
 *
 * This function checks to see if a seccomp() flag is supported by the system.
 * If the flag is supported zero is returned, negative values otherwise.
 *
 */
int sys_chk_seccomp_flag(int flag)
{
#ifdef HAVE_SECCOMP
	switch (flags) {
	case SECCOMP_FILTER_FLAG_TSYNC:
		return 0;
	default:
		return -EOPNOTSUPP;
	}
#else
	return -EOPNOTSUPP;
#endif /* HAVE_SECCOMP */
}

/**
 * Loads the filter into the kernel
 * @param col the filter collection
 *
 * This function loads the given seccomp filter context into the kernel.  If
 * the filter was loaded correctly, the kernel will be enforcing the filter
 * when this function returns.  Returns zero on success, negative values on
 * error.
 *
 */
int sys_filter_load(const struct db_filter_col *col)
{
	int rc;
	struct bpf_program *program = NULL;

	program = gen_bpf_generate(col);
	if (program == NULL)
		return -ENOMEM;

	/* attempt to set NO_NEW_PRIVS */
	if (col->attr.nnp_enable) {
		rc = prctl(PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0);
		if (rc < 0)
			goto filter_load_out;
	}

	/* load the filter into the kernel */
#ifdef HAVE_SECCOMP
	{
		int flags = 0;
		if (col->attr.tsync_enable)
			flags = SECCOMP_FILTER_FLAG_TSYNC;
		rc = seccomp(SECCOMP_SET_MODE_FILTER, flags, program);
		if (rc > 0 && col->attr.tsync_enable)
			/* always return -ESRCH if we fail to sync threads */
			errno = -ESRCH;
	}
#else
	rc = prctl(PR_SET_SECCOMP, SECCOMP_MODE_FILTER, program);
#endif /* HAVE_SECCOMP */

filter_load_out:
	/* cleanup and return */
	gen_bpf_release(program);
	if (rc < 0)
		return -errno;
	return 0;
}
