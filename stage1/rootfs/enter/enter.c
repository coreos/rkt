#define _GNU_SOURCE
#include <fcntl.h>
#include <limits.h>
#include <sched.h>
#include <stdio.h>
#include <stdlib.h>
#include <sys/stat.h>
#include <sys/types.h>
#include <unistd.h>

static int errornum;
#define exit_if(_cond, _fmt, _args...)			\
	errornum++;					\
	if(_cond) {					\
		fprintf(stderr, _fmt "\n", ##_args);	\
		exit(errornum);				\
	}

#define NAMESPACES		\
ns(CLONE_NEWIPC,  "ns/ipc")	\
ns(CLONE_NEWUTS,  "ns/uts")	\
ns(CLONE_NEWNET,  "ns/net")	\
ns(CLONE_NEWPID,  "ns/pid")	\
ns(CLONE_NEWNS,	  "ns/mnt")

static int pid;
static int openpidfd(char *which) {
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
	int	root_fd, work_fd, fd;

	exit_if((fp = fopen("pid", "r")) == NULL, "unable to open pid file");
	exit_if(fscanf(fp, "%i", &pid) != 1, "unable to read pid");
	fclose(fp);
	root_fd = openpidfd("root");
	work_fd = openpidfd("cwd");

#define ns(_typ, _nam)							\
	fd = openpidfd(_nam);						\
	exit_if(setns(fd, _typ), "unable to enter " _nam " namespace");
	NAMESPACES

	exit_if(fchdir(root_fd) < 0, "unable to chdir to container root");
	exit_if(chroot(".") < 0, "unable to chroot");
	exit_if(fchdir(work_fd) < 0, "unable to chdir to container cwd");
	exit_if(execv("/trampoline.sh", argv) == -1, "exec failed");
	return 0;
}
