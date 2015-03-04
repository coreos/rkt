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

/* Capsule is a minimal nspawn-like container executor,
 * heavily influenced by the systemd-nspawn code and overall container model.
 *
 * Desired differences of the moment:
 * - Readily built statically (important for pre-container/chroot execution)
 * - Execute normally on non-sysd hosts without LD_PRELOAD hacks
 * - --pid-file (needed for writing container pid)
 * - --keep-fd (needed for respecting RKT_LOCK_FD)
 * - Run-time suppression of pty allocation
 * - Run-time control over namespaces created (CLONE_NEWUSER too)
 * - Run-time control over the syscall blacklist (TODO)
 */

/* TODO(vc):
 * pivot_root vs. MS_MOVE target -> / && chroot, is there reason to use pivot_root()?
 * personality?
 * selinux?
 */

#include <errno.h>
#include <inttypes.h>
#include <fcntl.h>
#include <poll.h>
#include <sched.h>
#include <signal.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/mount.h>
#include <sys/prctl.h>
#include <sys/stat.h>
#include <sys/syscall.h>
#include <sys/types.h>
#include <unistd.h>
#include <sys/wait.h>

#include "args.h"
#include "blacklist.h"
#include "capabilities.h"
#include "fatal.h"
#include "fds.h"
#include "mounts.h"
#include "pty.h"

int			exit_err = 0;
static volatile int	exited;
static int		child_pid = -1;

/* SIGCHLD handler for parent */
static void parent_sigchld_handler(int sig)
{
	exited = 1;
}

/* SIGTERM/SIGINT/SIGQUIT forwarder,
 * propagate these to the child and let the usual SIGCHLD handling commence.
 *
 * If we are brutally killed (SIGKILL) or otherwise exit as parent, the child
 * will be killed with SIGKILL due to the PR_SET_PDEATHSIG prctl.
 *
 * XXX: Note there is technically a small window between the clone() and prctl()
 * where the parent could exit without the child being killed, but the window is
 * as small as possible (the prctl() is the first thing the child does).
 */
static void parent_sigfwd_handler(int sig)
{
	kill(child_pid, sig);
}

/* pre-clone setup signals for parent */
/* first we block everything except SIGCHLD */
static void parent_setup_signals_pre(void)
{
	sigset_t		mask, sigchld_mask;
	struct sigaction	sa = {
		.sa_handler = parent_sigchld_handler,
		.sa_flags = SA_NOCLDSTOP,
	};

	pexit_if(sigemptyset(&mask) == -1, "sigemptyset failed");
	pexit_if(sigaddset(&mask, SIGCHLD) == -1, "sigaddset failed");
	pexit_if(sigaddset(&mask, SIGWINCH) == -1, "sigaddset failed");
	pexit_if(sigaddset(&mask, SIGTERM) == -1, "sigaddset failed");
	pexit_if(sigaddset(&mask, SIGINT) == -1, "sigaddset failed");
	pexit_if(sigprocmask(SIG_BLOCK, &mask, NULL) == -1,
		 "sigprocmask block all failed");

	pexit_if(sigemptyset(&sigchld_mask) == -1, "sigemptyset failed");
	pexit_if(sigaddset(&sigchld_mask, SIGCHLD) == -1, "sigaddset failed");

	/* install handler and unblock SIGCHLD before calling clone() */
	pexit_if(sigaction(SIGCHLD, &sa, NULL) == -1,
		 "sigaction failed to install SIGCHLD handler");
	pexit_if(sigprocmask(SIG_UNBLOCK, &sigchld_mask, NULL) == -1,
		 "sigprocmask unblock sigchld failed");
}

/* post-clone setup signals for parent */
/* then we unblock SIGTERM, SIGINT, SIGQUIT */
static void parent_setup_signals_post(void)
{
	sigset_t		fwd_mask;
	struct sigaction	sa = {
		.sa_handler = parent_sigfwd_handler,
		.sa_flags = 0
	};

	pexit_if(sigemptyset(&fwd_mask) == -1, "sigemptyset failed");
	pexit_if(sigaddset(&fwd_mask, SIGTERM) == -1, "sigaddset failed");
	pexit_if(sigaddset(&fwd_mask, SIGINT) == -1, "sigaddset failed");
	pexit_if(sigaddset(&fwd_mask, SIGQUIT) == -1, "sigaddset failed");

	pexit_if(sigaction(SIGTERM, &sa, NULL) == -1,
		 "sigaction failed to install SIGTERM handler");
	pexit_if(sigaction(SIGINT, &sa, NULL) == -1,
		 "sigaction failed to install SIGINT handler");
	pexit_if(sigaction(SIGQUIT, &sa, NULL) == -1,
		 "sigaction failed to install SIGQUIT handler");

	pexit_if(sigprocmask(SIG_UNBLOCK, &fwd_mask, NULL) == -1,
		 "sigprocmask unblock fwd failed");
}

/* setup signals for child (unblocked and defaults) */
static void child_setup_signals(void)
{
        sigset_t	empty_mask;
	int		s;

	for(s = 1; s < _NSIG; s++) {
		struct sigaction sa = {
			.sa_handler = SIG_DFL,
			.sa_flags = 0
		};

		if(s == SIGKILL || s == SIGSTOP)
			continue;

		pexit_if(sigaction(s, &sa, NULL) == -1 && errno != EINVAL,
			 "failed to set default handler for %i", s);
	}

	pexit_if(sigemptyset(&empty_mask) == -1,
		 "sigemptyset failed");
	pexit_if(sigprocmask(SIG_SETMASK, &empty_mask, NULL) == -1,
		 "sigprocmask setmask empty failed");
}

/* write pid to pid_file */
static void write_pidfile(const char *pid_file, int pid)
{
	FILE	*pf;

	pexit_if((pf = fopen(pid_file, "w")) == NULL,
		 "error opening pid-file \"%s\"", pid_file);
	pexit_if(fprintf(pf, "%i\n", pid) < 0,
		 "error writing pid-file \"%s\"", pid_file);
	pexit_if(fclose(pf) != 0,
		 "error closing pid-file \"%s\"", pid_file);
}

/* turn tdir into a bind mount and chdir to it */
static void bind_target_chdir(const char *tdir)
{
	pexit_if(mount(NULL, "/", NULL, MS_REC|MS_SLAVE, NULL) == -1,
		 "failed to recursively slave-mount \"/\"");
	pexit_if(mount(tdir, tdir, "bind", MS_BIND|MS_REC, NULL) == -1,
		 "failed to convert \"%s\" into a bind mount", tdir);
	pexit_if(chdir(tdir) == -1,
		 "failed to chdir to \"%s\"", tdir);
}

/* minimally prepare the base root in CWD according to prepsteps.def */
static void prep_base_in_cwd(void)
{
#define mnt(_src, _tgt, _typ, _flags, _data)			\
	pexit_if(mount(_src, _tgt, _typ, _flags, _data) == -1,	\
		 "error mounting " #_tgt);
#define dir(_tgt, _mod)						\
	pexit_if(mkdir(_tgt, _mod) == -1,			\
		 "error creating directory " #_tgt);
#define nod(_tgt, _mod, _dev)					\
	pexit_if(mknod(_tgt, _mod, _dev) == -1,			\
		 "error creating device " #_tgt);
#define sym(_tgt, _nam)						\
	pexit_if(symlink(_tgt, _nam) == -1,			\
		"error creating symlink " #_nam);
#include "defs/prepsteps.def"
}

/* finalize our transition into target_dir */
static void pivot_target_dir(const char *td)
{
	/* TODO(vc): research merits of using pivot_root vs. chroot here */
	pexit_if(mount(td, "/", NULL, MS_MOVE, NULL) == -1,
		 "failed to MS_MOVE \"%s\" to \"/\"", td);
	pexit_if(chroot(".") == -1, "failed to chroot");
	pexit_if(chdir("/") == -1, "failed to chdir to \"/\"");
}

/* child_env returns the appropriate environment for the child's execve() */
static const char ** child_env(args_t *args, int n_listen_fds)
{
	int		cnt = 5;
	char		*term;
	static char	*env[] = {
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"HOME=/",		/* TODO(vc): derive from args? */
		"USER=root",		/* TODO(vc): change when --user gets added */
		"LOGNAME=root",
		"container=rocket",
		NULL, /* TERM */
		NULL, /* container_uuid */
		NULL, /* LISTEN_FDS */
		NULL, /* LISTEN_PID */
		NULL
	};

	/* TODO(vc): add --setenv like nspawn, if needed. */

	if((term = getenv("TERM")) != NULL) {
		exit_if(asprintf(&env[cnt++], "TERM=%s", term) == -1,
			"unable to create TERM env");
	}

	if(args->target_uuid) {
		exit_if(asprintf(&env[cnt++], "container_uuid=%s",
				 args->target_uuid) == -1,
			"unable to create uuid env");
	}

	if(n_listen_fds) {
		exit_if(asprintf(&env[cnt++], "LISTEN_FDS=%i", n_listen_fds) == -1,
			"unable to create LISTEN_FDS env");
		env[cnt++] = "LISTEN_PID=1";
	}

	return (const char **)env;
}

/* clone_child creates/implements the child process which becomes the container */
static void clone_child(args_t *args, const char *pts_name, int n_listen_fds)
{

	parent_setup_signals_pre();

	pexit_if((child_pid = syscall(__NR_clone,
				      SIGCHLD|args->namespaces, NULL)) == -1,
		 "unable to clone child process");

	if(child_pid) {
		/* parent returns */
		parent_setup_signals_post();
		return;
	}

	/* SIGKILL the child if the parent exits */
	pexit_if(prctl(PR_SET_PDEATHSIG, SIGKILL) == -1,
		 "error setting PDEATHSIG");

	child_setup_signals();

	if(!args->no_pty)
		pty_begin_session(pts_name);

	bind_target_chdir(args->target_dir);
	prep_base_in_cwd();

	if(args->no_pty && isatty(0)) {
		pexit_if((pts_name = ttyname(0)) == NULL,
			 "error determining controlling tty name");
	}
	/* (no_pty && !isatty(0)) leaves a bit of an awkward situation;
	 * /dev/console will be left with /dev/null's major,minor.
	 * TODO(vc): explore options to improve this, --no-pty is just
	 * sort of hacky but works perfectly fine for simple uses.
	 */
	pexit_if(pts_name && mount(pts_name, "dev/console", NULL,
				   MS_BIND, NULL) == -1,
		 "error binding \"%s\" to dev/console", pts_name);

	mounts_mount_all(args->target_dir);

	/* TODO(vc): setup boot id like nspawn does? */

	pivot_target_dir(args->target_dir);
	capabilities_drop(~(args->caps_kept & ~args->caps_dropped));
	pexit_if(sethostname(args->target_name,
			     strlen(args->target_name)) == -1,
		 "failed setting hostname to \"%s\"", args->target_name);

	blacklist_apply(args);
#if 0
	/* TODO(vc): nspawn does these */
	personality
	selinux
#endif
	pexit_if(execve(args->target_argv[0],
			(char * const *)args->target_argv,
			(char * const *)child_env(args, n_listen_fds)) == -1,
		 "execve of \"%s\" failed", args->target_argv[0]);
	/* XXX unreachable */
}

int main(int argc, const char *argv[])
{
	args_t	*args;
	int	ptm = -1;
	int	n_listen_fds = 0;
	char	*pts_name = NULL;

	args = args_handle(argc, argv);
	exit_if(geteuid(), "%s requires root privileges", argv[0]);

	/* TODO(vc): perform sanity checks on args->target_dir (exists, S_ISDIR...) */

	n_listen_fds = fds_get_listen_fds();
	fds_close_most(n_listen_fds, args->kept_fd);

	if(!args->no_pty)
		pty_alloc(&ptm, &pts_name);

	clone_child(args, pts_name, n_listen_fds);
	fds_close_rest(n_listen_fds);

	if(args->pid_file)
		write_pidfile(args->pid_file, child_pid);

	if(args->no_pty) {
		while(!exited)
			pause();
	} else {
		pty_manage(ptm, args->parent_isig, &exited);
	}

	kill(child_pid, SIGKILL);
	waitpid(child_pid, NULL, 0); // TODO(vc): collect and return child exit status

	return 0;
}
