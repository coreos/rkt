/**
 * Seccomp Library utility code for tests
 *
 * Copyright (c) 2012 Red Hat <eparis@redhat.com>
 * Author: Eric Paris <eparis@redhat.com>
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
#include <getopt.h>
#include <signal.h>
#include <stdio.h>
#include <string.h>
#include <unistd.h>
#include <sys/types.h>
#include <sys/stat.h>

#include <seccomp.h>

#include "util.h"

/**
 * SIGSYS signal handler
 * @param nr the signal number
 * @param info siginfo_t pointer
 * @param void_context handler context
 *
 * Simple signal handler for SIGSYS which exits with error code 161.
 *
 */
static void _trap_handler(int signal, siginfo_t *info, void *ctx)
{
	_exit(161);
}

/**
 * Parse the arguments passed to main
 * @param argc the argument count
 * @param argv the argument pointer
 * @param opts the options structure
 *
 * This function parses the arguments passed to the test from the command line.
 * Returns zero on success and negative values on failure.
 *
 */
int util_getopt(int argc, char *argv[], struct util_options *opts)
{
	int rc = 0;

	if (opts == NULL)
		return -EFAULT;

	memset(opts, 0, sizeof(*opts));
	while (1) {
		int c, option_index = 0;
		const struct option long_options[] = {
			{"bpf", no_argument, &(opts->bpf_flg), 1},
			{"pfc", no_argument, &(opts->bpf_flg), 0},
			{0, 0, 0, 0},
		};

		c = getopt_long(argc, argv, "bp",
				long_options, &option_index);
		if (c == -1)
			break;

		switch (c) {
		case 0:
			break;
		case 'b':
			opts->bpf_flg = 1;
			break;
		case 'p':
			opts->bpf_flg = 0;
			break;
		default:
			rc = -EINVAL;
			break;
		}
	}

	if (rc == -EINVAL || optind < argc) {
		fprintf(stderr, "usage %s: [--bpf,-b] [--pfc,-p]\n", argv[0]);
		rc = -EINVAL;
	}

	return rc;
}

/**
 * Output the filter in either BPF or PFC
 * @param opts the options structure
 * @param ctx the filter context
 *
 * This function outputs the seccomp filter to stdout in either BPF or PFC
 * format depending on the test paramaeters supplied by @opts.
 *
 */
int util_filter_output(const struct util_options *opts,
		       const scmp_filter_ctx ctx)
{
	int rc;

	if (opts == NULL)
		return -EFAULT;

	if (opts->bpf_flg)
		rc = seccomp_export_bpf(ctx, STDOUT_FILENO);
	else
		rc = seccomp_export_pfc(ctx, STDOUT_FILENO);

	return rc;
}

/**
 * Install a TRAP action signal handler
 *
 * This function installs the TRAP action signal handler and is based on
 * examples from Will Drewry and Kees Cook.  Returns zero on success, negative
 * values on failure.
 *
 */
int util_trap_install(void)
{
	struct sigaction signal_handler;
	sigset_t signal_mask;

	memset(&signal_handler, 0, sizeof(signal_handler));
	sigemptyset(&signal_mask);
	sigaddset(&signal_mask, SIGSYS);

	signal_handler.sa_sigaction = &_trap_handler;
	signal_handler.sa_flags = SA_SIGINFO;
	if (sigaction(SIGSYS, &signal_handler, NULL) < 0)
		return -errno;
	if (sigprocmask(SIG_UNBLOCK, &signal_mask, NULL))
		return -errno;

	return 0;
}

/**
 * Parse a filter action string into an action value
 * @param action the action string
 *
 * Parse a seccomp action string into the associated integer value.  Returns
 * the correct value on success, -1 on failure.
 *
 */
int util_action_parse(const char *action)
{
	if (action == NULL)
		return -1;

	if (strcasecmp(action, "KILL") == 0)
		return SCMP_ACT_KILL;
	else if (strcasecmp(action, "TRAP") == 0)
		return SCMP_ACT_TRAP;
	else if (strcasecmp(action, "ERRNO") == 0)
		return SCMP_ACT_ERRNO(163);
	else if (strcasecmp(action, "TRACE") == 0)
		return -1; /* not yet supported */
	else if (strcasecmp(action, "ALLOW") == 0)
		return SCMP_ACT_ALLOW;

	return -1;
}

/**
 * Write a string to a file
 * @param path the file path
 *
 * Open the specified file, write a string to the file, and close the file.
 * Return zero on success, negative values on error.
 *
 */
int util_file_write(const char *path)
{
	int fd;
	const char buf[] = "testing";
	ssize_t buf_len = strlen(buf);

	fd = open(path, O_WRONLY | O_CREAT, S_IRUSR | S_IWUSR);
	if (fd < 0)
		return errno;
	if (write(fd, buf, buf_len) < buf_len) {
		int rc = errno;
		close(fd);
		return rc;
	}
	if (close(fd) < 0)
		return errno;

	return 0;
}
