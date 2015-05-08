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
#include <dlfcn.h>
#include <fcntl.h>
#include <stdarg.h>
#include <stdlib.h>
#include <stdio.h>
#include <string.h>
#include <sys/stat.h>
#include <sys/syscall.h>
#include <sys/types.h>
#include <unistd.h>


/* hack to make systemd-nspawn execute on non-sysd systems:
 * - intercept lstat() so lstat of /run/systemd/system always succeeds and returns a directory
 * - intercept close() to prevent nspawn closing the rkt lock, set it to CLOEXEC instead
 * - intercept syscall(SYS_clone) to record the pod's pid
 */

#define ENV_LOCKFD	"RKT_LOCK_FD"
#define PIDFILE_TMP	"pid.tmp"
#define PIDFILE		"pid"

static int (*libc_lxstat)(int, const char *, struct stat *);
static int (*libc_close)(int);
static long (*libc_syscall)(long number, ...);
static int lock_fd = -1;

static __attribute__((constructor)) void wrapper_init(void)
{
	char *env;
	if((env = getenv(ENV_LOCKFD)))
		lock_fd = atoi(env);
	libc_lxstat = dlsym(RTLD_NEXT, "__lxstat");
	libc_close = dlsym(RTLD_NEXT, "close");
	libc_syscall = dlsym(RTLD_NEXT, "syscall");
}

int __lxstat(int ver, const char *path, struct stat *stat)
{
	int ret = libc_lxstat(ver, path, stat);

	if(ret == -1 && !strcmp(path, "/run/systemd/system/")) {
		stat->st_mode = S_IFDIR;
		ret = 0;
	}

	return ret;
}

int close(int fd)
{
	if(lock_fd != -1 && fd == lock_fd)
		return fcntl(fd, F_SETFD, FD_CLOEXEC);

	return libc_close(fd);
}

long syscall(long number, ...)
{
	unsigned long	clone_flags;
	va_list		ap;
	long		ret;

	/* XXX: we're targeting systemd-nspawn with this shim, its only syscall() use is __NR_clone */
	if(number != __NR_clone)
		return -1;

	va_start(ap, number);
	clone_flags = va_arg(ap, unsigned long);
	va_end(ap);

	ret = libc_syscall(number, clone_flags, NULL);

	if(ret > 0) {
		int fd;
		/* in parent; try record the pod's pid */
		if((fd = open(PIDFILE_TMP, O_CREAT|O_WRONLY|O_SYNC, 0640)) != -1) {
			int	len;
			char	buf[20];

			if((len = snprintf(buf, sizeof(buf), "%li\n", ret)) != -1)
				if(write(fd, buf, len) == len)
					rename(PIDFILE_TMP, PIDFILE);

			libc_close(fd);
		}
	}

	return ret;
}
