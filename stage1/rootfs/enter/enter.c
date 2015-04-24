// Copyright 2014 CoreOS, Inc.
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
#define exit_if(_cond, _fmt, _args...)                                    \
	errornum++;                                                       \
	if(_cond) {                                                       \
		fprintf(stderr, _fmt ": %s\n", ##_args, strerror(errno)); \
		exit(errornum);                                           \
	}

static int openpidfd(int pid, char *which) {
	char	path[PATH_MAX];
	int	fd;
	exit_if(snprintf(path, sizeof(path),
			 "/proc/%i/%s", pid, which) == sizeof(path),
		"path overflow");
	exit_if((fd = open(path, O_RDONLY|O_CLOEXEC)) == -1,
		"unable to open \"%s\"", path);
	return fd;
}

int main(int argc, char *argv[])
{
	FILE	*fp;
	int	fd;
	int	pid;
	pid_t	child;
	int	status;
	int	root_fd;

	exit_if(argc < 3,
		"Usage: %s imageid cmd [args...]", argv[0])

	/* We start in the pod root, where "pid" should be. */
	exit_if((fp = fopen("pid", "r")) == NULL,
		"unable to open pid file");
	exit_if(fscanf(fp, "%i", &pid) != 1,
		"unable to read pid");
	fclose(fp);
	root_fd = openpidfd(pid, "root");

#define ns(_typ, _nam)							\
	fd = openpidfd(pid, _nam);					\
	exit_if(setns(fd, _typ), "unable to enter " _nam " namespace");

#if 0
	/* TODO(vc): Nspawn isn't employing CLONE_NEWUSER, disabled for now */
	ns(CLONE_NEWUSER, "ns/user");
#endif
	ns(CLONE_NEWIPC,  "ns/ipc");
	ns(CLONE_NEWUTS,  "ns/uts");
	ns(CLONE_NEWNET,  "ns/net");
	ns(CLONE_NEWPID,  "ns/pid");
	ns(CLONE_NEWNS,	  "ns/mnt");

	exit_if(fchdir(root_fd) < 0,
		"unable to chdir to pod root");
	exit_if(chroot(".") < 0,
		"unable to chroot");
	exit_if(close(root_fd) == -1,
		"unable to close root_fd");

	/* Fork is required to realize consequence of CLONE_NEWPID */
	exit_if(((child = fork()) == -1),
		"unable to fork");

	if(child == 0) {
		char		root[PATH_MAX];
		char		env[PATH_MAX];
		char		*args[argc + 4];
		int		i;

		/* Child goes on to execute /diagexec */

		exit_if(snprintf(root, sizeof(root),
				 "/opt/stage2/%s/rootfs", argv[1]) == sizeof(root),
			"root path overflow");

		exit_if(snprintf(env, sizeof(env),
				 "/rkt/env/%s", argv[1]) == sizeof(env),
			"env path overflow");

		args[0] = "/diagexec";
		args[1] = root;
		args[2] = "/";	/* TODO(vc): plumb this into app.WorkingDirectory */
		args[3] = env;
		args[4] = "0"; /* uid */
		args[5] = "0"; /* gid */
		for(i = 2; i < argc; i++) {
			args[i + 4] = argv[i];
		}
		args[i + 4] = NULL;

		exit_if(execv(args[0], args) == -1,
			"exec failed");
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
