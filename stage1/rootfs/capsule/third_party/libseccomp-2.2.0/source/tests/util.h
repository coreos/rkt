/**
 * Seccomp Library utility code for tests
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

#ifndef _UTIL_TEST_H
#define _UTIL_TEST_H

struct util_options {
	int bpf_flg;
};

int util_getopt(int argc, char *argv[], struct util_options *opts);

int util_filter_output(const struct util_options *opts,
		       const scmp_filter_ctx ctx);

int util_trap_install(void);

int util_action_parse(const char *action);

int util_file_write(const char *path);

#endif
