// Copyright 2014 The rkt Authors
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

#define _GNU_SOURCE
#include <errno.h>
#include <fcntl.h>
#include <limits.h>
#include <sched.h>
#include <signal.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>
#include <sys/types.h>
#include <sys/wait.h>
#include <unistd.h>

static int errornum;
#define exit_if(_cond, _fmt, _args...)				\
	errornum++;						\
	if(_cond) {						\
		fprintf(stderr, _fmt "\n", ##_args);		\
		exit(errornum);					\
	}
#define pexit_if(_cond, _fmt, _args...)				\
	exit_if(_cond, _fmt ": %s", ##_args, strerror(errno))

static int openpidfd(int pid, char *which) {
	char	path[PATH_MAX];
	int	fd;
	exit_if(snprintf(path, sizeof(path),
			 "/proc/%i/%s", pid, which) == sizeof(path),
		"Path overflow");
	pexit_if((fd = open(path, O_RDONLY|O_CLOEXEC)) == -1,
		"Unable to open \"%s\"", path);
	return fd;
}

int main(int argc, char *argv[])
{
	int	fd;
	int	pid;
	pid_t	child;
	int	status;
	int	root_fd;

	/* The parameters list is specified in
	 * Documentation/devel/stage1-implementors-guide.md */
	exit_if(argc < 4,
		"Usage: %s pid imageid cmd [args...]", argv[0])

	pid = atoi(argv[1]);
	root_fd = openpidfd(pid, "root");

#define ns(_typ, _nam)							\
	fd = openpidfd(pid, _nam);					\
	pexit_if(setns(fd, _typ), "Unable to enter " _nam " namespace");

#if 0
	/* TODO(vc): Nspawn isn't employing CLONE_NEWUSER, disabled for now */
	ns(CLONE_NEWUSER, "ns/user");
#endif
	ns(CLONE_NEWIPC,  "ns/ipc");
	ns(CLONE_NEWUTS,  "ns/uts");
	ns(CLONE_NEWNET,  "ns/net");
	ns(CLONE_NEWPID,  "ns/pid");
	ns(CLONE_NEWNS,	  "ns/mnt");

	pexit_if(fchdir(root_fd) < 0,
		"Unable to chdir to pod root");
	pexit_if(chroot(".") < 0,
		"Unable to chroot");
	pexit_if(close(root_fd) == -1,
		"Unable to close root_fd");

	/* Fork is required to realize consequence of CLONE_NEWPID */
	pexit_if(((child = fork()) == -1),
		"Unable to fork");

/* some stuff make the argv->args copy less cryptic */
#define ENTER_ARGV_FWD_OFFSET		3
#define DIAGEXEC_ARGV_FWD_OFFSET	6
#define args_fwd_idx(_idx) \
	((_idx - ENTER_ARGV_FWD_OFFSET) + DIAGEXEC_ARGV_FWD_OFFSET)

	if(child == 0) {
		char		root[PATH_MAX];
		char		env[PATH_MAX];
		char		*args[args_fwd_idx(argc) + 1 /* NULL terminator */];
		int		i;

		/* Child goes on to execute /diagexec */

		exit_if(snprintf(root, sizeof(root),
				 "/opt/stage2/%s/rootfs", argv[2]) == sizeof(root),
			"Root path overflow");

		exit_if(snprintf(env, sizeof(env),
				 "/rkt/env/%s", argv[2]) == sizeof(env),
			"Env path overflow");

		args[0] = "/diagexec";
		args[1] = root;
		args[2] = "/";	/* TODO(vc): plumb this into app.WorkingDirectory */
		args[3] = env;
		args[4] = "0"; /* uid */
		args[5] = "0"; /* gid */
		for(i = ENTER_ARGV_FWD_OFFSET; i < argc; i++) {
			args[args_fwd_idx(i)] = argv[i];
		}
		args[args_fwd_idx(i)] = NULL;

		pexit_if(execv(args[0], args) == -1,
			"Exec failed");
	}

	/* Wait for child, nsenter-like */
	for(;;) {
		if(waitpid(child, &status, WUNTRACED) == pid &&
		   (WIFSTOPPED(status))) {
			kill(getpid(), SIGSTOP);
			/* the above stops us, upon receiving SIGCONT we'll
			 * continue here and inform our child */
			kill(child, SIGCONT);
		} else {
			break;
		}
	}

	if(WIFEXITED(status)) {
		exit(WEXITSTATUS(status));
	} else if(WIFSIGNALED(status)) {
		kill(getpid(), WTERMSIG(status));
	}

	return EXIT_FAILURE;
}
