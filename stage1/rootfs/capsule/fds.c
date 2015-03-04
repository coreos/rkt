// Copyright 2015 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

#include <errno.h>
#include <fcntl.h>
#include <limits.h>
#include <stdlib.h>
#include <sys/resource.h>
#include <sys/time.h>
#include <unistd.h>

#include "fatal.h"

#define SYSD_LISTEN_START 3

/* return systemd-style socket activation LISTEN_FDS */
int fds_get_listen_fds(void)
{
	int	n_fds = 0;
	char	*env;

	env = getenv("LISTEN_PID");
	if(env) {
		unsigned long ul;

		errno = 0;
		ul = strtoul(env, NULL, 10);
		pexit_if(errno, "LISTEN_PID strtoul failed");

		if(ul == getpid()) {
			env = getenv("LISTEN_FDS");
			errno = 0;
			ul = strtoul(env, NULL, 10);
			pexit_if(errno, "LISTEN_FDS strtoul failed");
			pexit_if(ul > (INT_MAX - SYSD_LISTEN_START),
				 "LISTEN_FDS overflow: %lu", ul);
			n_fds = ul;
		}
	}

	return n_fds;
}

/* close file descriptors, skipping std in/out/err and listen_fds, setting O_CLOEXEC on kept_fd */
void fds_close_most(int n_listen_fds, int kept_fd)
{
	struct	rlimit nf;
	int	fd;

	pexit_if(getrlimit(RLIMIT_NOFILE, &nf) == -1,
		 "error getting RLIMIT_NOFILE");

	for(fd = SYSD_LISTEN_START + n_listen_fds; fd < nf.rlim_max; fd++) {
		if(fd == kept_fd) {
			fcntl(fd, F_SETFD, (int)FD_CLOEXEC);
			continue;
		}
		pexit_if(close(fd) == -1 && errno != EBADF,
			 "error closing fd %i", fd);
	}
}

/* close the LISTEN_FDS if there are any, the parent doesn't need to keep them around after clone */
void fds_close_rest(int n_listen_fds)
{
	int fd;

	for(fd = SYSD_LISTEN_START; fd < SYSD_LISTEN_START + n_listen_fds; fd++) {
		pexit_if(close(fd) == -1, "error closing listen_fd %i", fd);
	}
}
