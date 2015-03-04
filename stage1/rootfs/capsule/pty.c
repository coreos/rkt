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

#include <fcntl.h>
#include <poll.h>
#include <stdlib.h>
#include <sys/ioctl.h>
#include <termios.h>
#include <unistd.h>

#include "fatal.h"

/* allocate a pseudo-terminal and return the master fd and slave name */
void pty_alloc(int *ptm_p, char **pts_name_p)
{
	int	ptm;
	char	*pts_name;

	pexit_if((ptm = posix_openpt(O_RDWR|O_NOCTTY|O_CLOEXEC)) == -1,
		 "error opening pseudo-terminal master");
	pexit_if((pts_name = ptsname(ptm)) == NULL,
		 "error getting pseudo-terminal slave name");
	pexit_if(unlockpt(ptm) == -1,
		 "error unlocking pseudo-terminal master");

	*ptm_p = ptm;
	*pts_name_p = pts_name;
}

/* open the slave pty and begin a new session,
 * stdin/stdout/stderr are left attached to the opened slave,
 * the slave is also made the controlling tty so control-sequence signals have someplace to go.
 */
void pty_begin_session(const char *pts_name)
{
	int		pts;

	pexit_if(close(0) == -1, "error closing stdin");
	pexit_if(close(1) == -1, "error closing stdout");
	pexit_if(close(2) == -1, "error closing stderr");

	pexit_if((pts = open(pts_name, O_RDWR)) == -1 || pts != 0,
		 "error opening pty slave \"%s\"",
		 pts_name);
	pexit_if(dup2(pts, 1) != 1,
		 "error duplicating pty slave (stdout)");
	pexit_if(dup2(pts, 2) != 2,
		 "error duplicating pty slave (stderr)");

	pexit_if(setsid() == -1,
		 "error creating new session");
	pexit_if(ioctl(pts, TIOCSCTTY, 0) == -1,
		 "error setting pty slave as controlling tty");
}

typedef struct _rawtio_t {
	struct termios	orig;
	int		fd;
} rawtio_t;

/* convert a terminal on fd to raw mode */
static void rawtio_get(int fd, int parent_isig, rawtio_t *rtio)
{
	if(tcgetattr(fd, &rtio->orig) >= 0) {
		struct termios raw;

		raw = rtio->orig;
		rtio->fd = fd;

		cfmakeraw(&raw);
		if(parent_isig) {
			raw.c_lflag |= (rtio->orig.c_lflag & ISIG);
		}
		pexit_if(tcsetattr(fd, TCSANOW, &raw) == -1,
			 "unable to set raw mode on fd %i", fd);
	} else {
		rtio->fd = -1;
	}
}

/* restore a terminal fd that has been made raw with rawtio_get() */
static void rawtio_put(rawtio_t *rtio)
{
	if(rtio->fd == -1)
		return;

	pexit_if(tcsetattr(rtio->fd, TCSANOW, &rtio->orig) == -1,
		 "unable to restore termio of fd %i", rtio->fd);
}

/* copy bytes from -> to, to is expected to block, forward EOF via close() */
static ssize_t copy(int from, int to)
{
	ssize_t	len;
	char	buf[8192];

	/* TODO(vc): make this more robust */
	len = read(from, buf, sizeof(buf));
	if(len == -1)
		return -1;
	else if(len == 0)
		close(to);
	else if(write(to, &buf, len) != len)
		return -1;

	return len;
}

/* copy data to/from the pty master */
void pty_manage(int ptm, int parent_isig, volatile int *exited_flag)
{
	rawtio_t	stdin_rtio, stdout_rtio;
	struct pollfd	pfds[2] = {
		{ .fd = 0,	.events = POLLIN|POLLHUP },
		{ .fd = ptm,	.events = POLLIN|POLLHUP }
	};

	/* TODO(vc): on SIGWINCH: TIOCGWINSZ(1)->TIOCSWINSZ(ptm), which will
	 * effectively propagate SIGWINCH to the slave side.
	 *
	 * When stdin is a tty which we make raw, simply forwarding the control
	 * sequences via ptm will result in signal emission from the slave if
	 * the pty is left configured to generate signals on those control
	 * sequences, and the parent won't be receiving signals from terminal
	 * control sequences due to the raw mode.
	 *
	 * Explicit forwarding of SIGINT/SIGTERM/SIGQUIT signals should still be
	 * performed to cover scenarios where these signals are simply delivered
	 * to the parent capsule process or for --no-pty which won't enter
	 * pty_manage() at all meaning no raw mode.
	 *
	 * When the child gives up its controlling tty (systemd?), merely
	 * propagating control sequences to the slave pty produces uninteresting
	 * results because the signals go undelivered.  In this case it's more
	 * useful to keep the capsule process receiving the terminal-generated
	 * signals and let the signal forwarding deliver them to the container.
	 * This is activated with --parent-isig, and is used in the rocket
	 * stage1 integration so ^C triggers SIGINT for systemd.
	 */

	rawtio_get(0, parent_isig, &stdin_rtio);
	rawtio_get(1, parent_isig, &stdout_rtio);

	/* parent manages the ptm until SIGCHLD arrives, which sets
	 * *exited_flag.  SIGTERM/SIGINT may be received and simply forwarded to
	 * the child for potentially orderly exit, so here we just tolerate
	 * interrupts and always check *exited_flag.
	 */
	while(!(*exited_flag)) {
		int ret = poll(pfds, 2, -1);
		if(ret == -1) {
			if(errno == EINTR)
				continue;
			break; 
		}

		if(pfds[0].revents) {
			if(copy(pfds[0].fd, pfds[1].fd) == -1) {
				if(errno == EINTR)
					continue;
				break;
			}
		}

		if(pfds[1].revents) {
			if(copy(pfds[1].fd, 1) == -1) {
				if(errno == EINTR)
					continue;
				break;
			}
		}
	}

	rawtio_put(&stdout_rtio);
	rawtio_put(&stdin_rtio);
}
