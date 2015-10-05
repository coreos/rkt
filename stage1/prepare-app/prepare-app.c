// Copyright 2015 The rkt Authors
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
#include <string.h>
#include <stdio.h>
#include <stdlib.h>
#include <sys/mount.h>
#include <sys/stat.h>
#include <sys/types.h>
#include <unistd.h>
#include <fcntl.h>

static int exit_err;
#define exit_if(_cond, _fmt, _args...)				\
	exit_err++;						\
	if(_cond) {						\
		fprintf(stderr, "Error: " _fmt "\n", ##_args);	\
		exit(exit_err);					\
	}
#define pexit_if(_cond, _fmt, _args...)				\
	exit_if(_cond, _fmt ": %s", ##_args, strerror(errno))

#define nelems(_array) \
	(sizeof(_array) / sizeof(_array[0]))

typedef struct _dir_op_t {
	const char	*name;
	mode_t		mode;
} dir_op_t;

typedef struct _mount_point_t {
	const char	*source;
	const char	*target;
	const char	*type;
	const char	*options;
	unsigned long	flags;
} mount_point;

#define dir(_name, _mode) \
	{ .name = _name, .mode = _mode }

int getmachinename(char *machine) {
	static const int machineid_size = 32; // 0123456789abcdef0123456789ab
	static const int machinename_size = machineid_size + 8; // rkt-01234567-89ab-cdef-0123-456789ab
	int fd;
	char buf[machineid_size + 1];
	ssize_t r;

	if ((fd = open("/etc/machine-id", O_RDONLY)) == -1)
		return -1;

	if ((r = read(fd, buf, machineid_size)) != machineid_size) {
		errno = EINVAL;
		return -1;
	}

	if (close(fd) != 0)
		return -1;

	if ((machine = malloc(machinename_size + 1)) == NULL)
		return -1;

	if (snprintf(machine, machinename_size, "rkt-%.8s-%.4s-%.4s-%.4s-%.8s",
		buf, buf+8, buf+12, buf+16, buf+20) >= machinename_size) {
		errno = EINVAL;
		return -1;
	}

	machine[machinename_size] = '\0';

	return 0;
}

int main(int argc, char *argv[])
{
	static const char *unlink_paths[] = {
		"dev/ptmx",
		"dev/shm",
		NULL
	};
	static const dir_op_t dirs[] = {
		dir("dev",	0755),
		dir("dev/net",	0755),
		dir("dev/pts",	0755),
		dir("dev/shm",	0755),
		dir("etc",	0755),
		dir("proc",	0755),
		dir("sys",	0755),
		dir("tmp",	01777),
	};
	static const char *devnodes[] = {
		"/dev/console",
		"/dev/full",
		"/dev/net/tun",
		"/dev/null",
		"/dev/random",
		"/dev/tty",
		"/dev/urandom",
		"/dev/zero",
		NULL
	};
	static const mount_point mount_table[] = {
		{ "/dev/pts", "/dev/pts", "bind", NULL, MS_BIND },
		{ "/dev/shm", "/dev/shm", "bind", NULL, MS_BIND },
		{ "/proc", "/proc", "bind", NULL, MS_BIND|MS_REC },
		{ "/sys", "/sys", "bind", NULL, MS_BIND|MS_REC },
	};
	const char *root;
	int rootfd;
	char to[4096];
	int i;

	exit_if(argc < 2,
		"Usage: %s /path/to/root", argv[0]);

	root = argv[1];

	/* Make stage2's root a mount point. Chrooting an application in a
	 * directory which is not a mount point is not nice because the
	 * application would not be able to remount "/" it as private mount.
	 * This allows Docker to run inside rkt.
	 * The recursive flag is to preserve volumes mounted previously by
	 * systemd-nspawn via "rkt run -volume".
	 * */
	pexit_if(mount(root, root, "bind", MS_BIND | MS_REC, NULL) == -1,
			"Make / a mount point failed");

	rootfd = open(root, O_DIRECTORY | O_CLOEXEC);
	pexit_if(rootfd < 0,
		"Failed to open directory \"%s\"", root);

	/* Some images have annoying symlinks that are resolved as dangling
	 * links before the chroot in stage1. E.g. "/dev/shm" -> "/run/shm"
	 * Just remove the symlinks.
         */
	for (i = 0; unlink_paths[i]; i++) {
		pexit_if(unlinkat(rootfd, unlink_paths[i], 0) != 0
			 && errno != ENOENT && errno != EISDIR,
			 "Failed to unlink \"%s\"", unlink_paths[i])
	}

	/* Create the directories */
	umask(0);
	for (i = 0; i < nelems(dirs); i++) {
		const dir_op_t *d = &dirs[i];
		pexit_if(mkdirat(rootfd, d->name, d->mode) == -1 &&
			 errno != EEXIST,
			"Failed to create directory \"%s/%s\"", root, d->name);
	}

	/* Create basic /etc/hosts if it doesn't exist */
	if (faccessat(rootfd, "etc/hosts", F_OK, AT_EACCESS) != 0) {
		char buf[128];
		int hostsfd;
		char *machine;

		hostsfd = openat(rootfd, "etc/hosts", O_RDWR|O_CREAT, 0644);
		pexit_if(hostsfd < 0,
			"Failed to open \"%s/etc/hosts\"", root);

		pexit_if(getmachinename(machine) != 0,
			"Failed to get machine name");

		exit_if(snprintf(buf, sizeof(buf), "%s\t%s\t%s\t%s\n", "127.0.0.1",
					machine, "localhost", "localhost.localdomain") >= sizeof(buf),
				"/etc/hosts line too long: \"%s\"", buf);

		free(machine);

		pexit_if(write(hostsfd, buf, strlen(buf)) != strlen(buf),
			"Failed to write to \"%s\"", to);

		pexit_if(close(hostsfd) != 0,
			"Failed to close \"%s\"", to);
	}

	close(rootfd);

	/* systemd-nspawn already creates few /dev entries in the container
	 * namespace: copy_devnodes()
	 * http://cgit.freedesktop.org/systemd/systemd/tree/src/nspawn/nspawn.c?h=v219#n1345
	 *
	 * But they are not visible by the apps because they are "protected" by
	 * the chroot.
	 *
	 * Bind mount them individually over the chroot border.
	 *
	 * Do NOT bind mount the whole directory /dev because it would shadow
	 * potential individual bind mount by stage0 ("rkt run --volume...").
	 *
	 * Do NOT use mknod, it would not work for /dev/console because it is
	 * a bind mount to a pts and pts device nodes only work when they live
	 * on a devpts filesystem.
	 */
	for (i = 0; devnodes[i]; i++) {
		const char *from = devnodes[i];
		int fd;

		/* If the file does not exist, skip it. It might be because
		 * the kernel does not provide it (e.g. kernel compiled without
		 * CONFIG_TUN) or because systemd-nspawn does not provide it
		 * (/dev/net/tun is not available with systemd-nspawn < v217
		 */
		if (access(from, F_OK) != 0)
			continue;

		exit_if(snprintf(to, sizeof(to), "%s%s", root, from) >= sizeof(to),
			"Path too long: \"%s\"", to);

		/* The mode does not matter: it will be bind-mounted over.
		 */
		fd = open(to, O_WRONLY|O_CREAT|O_CLOEXEC|O_NOCTTY, 0644);
		if (fd != -1)
			close(fd);

		pexit_if(mount(from, to, "bind", MS_BIND, NULL) == -1,
				"Mounting \"%s\" on \"%s\" failed", from, to);
	}

	/* Bind mount directories */
	for (i = 0; i < nelems(mount_table); i++) {
		const mount_point *mnt = &mount_table[i];

		exit_if(snprintf(to, sizeof(to), "%s/%s", root, mnt->target) >= sizeof(to),
			"Path too long: \"%s\"", to);
		pexit_if(mount(mnt->source, to, mnt->type,
			       mnt->flags, mnt->options) == -1,
				"Mounting \"%s\" on \"%s\" failed", mnt->source, to);
	}

	/* /dev/ptmx -> /dev/pts/ptmx */
	exit_if(snprintf(to, sizeof(to), "%s/dev/ptmx", root) >= sizeof(to),
		"Path too long: \"%s\"", to);
	pexit_if(symlink("/dev/pts/ptmx", to) == -1,
		"Failed to create /dev/ptmx symlink");

	return EXIT_SUCCESS;
}
