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

#include <errno.h>
#include <fcntl.h>
#include <inttypes.h>
#include <limits.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/mman.h>
#include <sys/mount.h>
#include <sys/stat.h>
#include <sys/types.h>
#include <unistd.h>

#include "elf.h"

static int exit_err;
#define exit_if(_cond, _fmt, _args...)				\
	exit_err++;						\
	if(_cond) {						\
		fprintf(stderr, "Error: " _fmt "\n", ##_args);	\
		exit(exit_err);					\
	}
#define pexit_if(_cond, _fmt, _args...)				\
	exit_if(_cond, _fmt ": %s", ##_args, strerror(errno))

#define MAX_DIAG_DEPTH 10
#define MIN(_a, _b) (((_a) < (_b)) ? (_a) : (_b))

static void map_file(const char *path, int prot, int flags, struct stat *st, void **map)
{
	int fd;

	pexit_if((fd = open(path, O_RDONLY)) == -1,
		"Unable to open \"%s\"", path);
	pexit_if(fstat(fd, st) == -1,
		"Cannot stat \"%s\"", path);
	exit_if(!S_ISREG(st->st_mode), "\"%s\" is not a regular file", path);
	pexit_if(!(*map = mmap(NULL, st->st_size, prot, flags, fd, 0)),
		"Mmap of \"%s\" failed", path);
	pexit_if(close(fd) == -1,
		"Close of %i [%s] failed", fd, path);
}

static void diag(const char *exe)
{
	static const uint8_t	elf[] = {0x7f, 'E', 'L', 'F'};
	static const uint8_t	shebang[] = {'#','!'};
	static int		diag_depth;
	struct stat		st;
	const uint8_t		*mm;
	const char		*itrp = NULL;

	map_file(exe, PROT_READ, MAP_SHARED, &st, (void **)&mm);
	exit_if(!((S_IXUSR|S_IXGRP|S_IXOTH) & st.st_mode),
		"\"%s\" is not executable", exe)

	if(st.st_size >= sizeof(shebang) &&
	   !memcmp(mm, shebang, sizeof(shebang))) {
		const uint8_t	*nl;
		int		maxlen = MIN(PATH_MAX, st.st_size - sizeof(shebang));
		/* TODO(vc): EOF-terminated shebang lines are technically possible */
		exit_if(!(nl = memchr(&mm[sizeof(shebang)], '\n', maxlen)),
			"Shebang line too long");
		pexit_if(!(itrp = strndup((char *)&mm[sizeof(shebang)], (nl - mm) - 2)),
			"Failed to dup interpreter path");
	} else if(st.st_size >= sizeof(elf) &&
		  !memcmp(mm, elf, sizeof(elf))) {
		uint64_t	(*lget)(const uint8_t *) = NULL;
		uint32_t	(*iget)(const uint8_t *) = NULL;
		uint16_t	(*sget)(const uint8_t *) = NULL;
		const void	*phoff = NULL, *phesz = NULL, *phecnt = NULL;
		const uint8_t	*ph = NULL;
		int		i, phreloff, phrelsz;

		exit_if(mm[ELF_VERSION] != 1,
			"Unsupported ELF version: %hhx", mm[ELF_VERSION]);

		/* determine which accessors to use and where */
		if(mm[ELF_BITS] == ELF_BITS_32) {
			if(mm[ELF_ENDIAN] == ELF_ENDIAN_LITL) {
				lget = le32_lget;
				sget = le_sget;
				iget = le_iget;
			} else if(mm[ELF_ENDIAN] == ELF_ENDIAN_BIG) {
				lget = be32_lget;
				sget = be_sget;
				iget = be_iget;
			}
			phoff = &mm[ELF32_PHT_OFF];
			phesz = &mm[ELF32_PHTE_SIZE];
			phecnt = &mm[ELF32_PHTE_CNT];
			phreloff = ELF32_PHE_OFF;
			phrelsz = ELF32_PHE_SIZE;
		} else if(mm[ELF_BITS] == ELF_BITS_64) {
			if(mm[ELF_ENDIAN] == ELF_ENDIAN_LITL) {
				lget = le64_lget;
				sget = le_sget;
				iget = le_iget;
			} else if(mm[ELF_ENDIAN] == ELF_ENDIAN_BIG) {
				lget = be64_lget;
				sget = be_sget;
				iget = be_iget;
			}
			phoff = &mm[ELF64_PHT_OFF];
			phesz = &mm[ELF64_PHTE_SIZE];
			phecnt = &mm[ELF64_PHTE_CNT];
			phreloff = ELF64_PHE_OFF;
			phrelsz = ELF64_PHE_SIZE;
		}

		exit_if(!lget, "Unsupported ELF format");

		if(!phoff) /* program header may be absent, don't make it an error */
			return;

		/* TODO(vc): sanity checks on values before using them */
		for(ph = &mm[lget(phoff)], i = 0; i < sget(phecnt); i++, ph += sget(phesz)) {
			if(iget(ph) == ELF_PT_INTERP) {
				itrp = strndup((char *)&mm[lget(&ph[phreloff])], lget(&ph[phrelsz]));
				break;
			}
		}
	} else {
		exit_if(1, "Unsupported file type");
	}

	exit_if(!itrp, "Unable to determine interpreter for \"%s\"", exe);
	exit_if(*itrp != '/', "Path must be absolute: \"%s\"", itrp);
	exit_if(++diag_depth > MAX_DIAG_DEPTH,
		"Excessive interpreter recursion, giving up");
	diag(itrp);
}

/* Read environment from env and make it our own. */
/* The environment file must exist, may be empty, and is expected to be of the format:
 * key=value\0key=value\0...
 */
static void load_env(const char *env)
{
	struct stat		st;
	char			*map, *k, *v;
	typeof(st.st_size)	i;

	map_file(env, PROT_READ|PROT_WRITE, MAP_PRIVATE, &st, (void **)&map);

	pexit_if(clearenv() != 0,
		"Unable to clear environment");

	if(!st.st_size)
		return;

	map[st.st_size - 1] = '\0'; /* ensure the mapping is null-terminated */

	for(i = 0; i < st.st_size;) {
		k = &map[i];
		i += strlen(k) + 1;
		exit_if((v = strchr(k, '=')) == NULL,
			"Malformed environment entry: \"%s\"", k);
		/* a private writable map is used permitting s/=/\0/ */
		*v = '\0';
		v++;
		pexit_if(setenv(k, v, 1) == -1,
			"Unable to set env variable: \"%s\"=\"%s\"", k, v);
	}
}

static void setup_dev(const char *root)
{
	static const char *dirs[] = {
		"/dev",
		"/dev/net",
		NULL
	};
	static const char *devnodes[] = {
		"/dev/null",
		"/dev/zero",
		"/dev/full",
		"/dev/random",
		"/dev/urandom",
		"/dev/tty",
		"/dev/net/tun",
		"/dev/console",
		NULL
	};
	int i;

	/* First, create the directories */
	for (i = 0; dirs[i]; i++) {
		const char *dir = dirs[i];
		char path[4096];

		snprintf(path, sizeof(path) - 1, "%s%s", root, dir);
		/* No need to check the error: the directory might already
		 * exist in the ACI. Errors will be catched by the mounts
		 * below anyway.
		 */
		mkdir(path, 0755);
	}

	/* systemd-nspawn already creates few /dev entries in the container
	 * namespace: copy_devnodes()
	 * http://cgit.freedesktop.org/systemd/systemd/tree/src/nspawn/nspawn.c#n1345
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
		char to[4096];
		int fd;

		/* If the file does not exist, skip it. It might be because
		 * the kernel does not provide it (e.g. kernel compiled without
		 * CONFIG_TUN) or because systemd-nspawn does not provide it
		 * (/dev/net/tun is not available with systemd-nspawn < v217
		 */
                if (access(from, F_OK) != 0)
			continue;

		snprintf(to, sizeof(to) - 1, "%s%s", root, from);

		/* Also, if the target file already exists, just skip the bind
		 * mount: it might come from the ACI, might have been setup by
		 * stage0 ("rkt run --volume...") or the bind mount might have
		 * been done previously ("systemctl restart app").
		 */
		fd = open(to, O_WRONLY|O_CREAT|O_CLOEXEC|O_NOCTTY, 0644);
		if (fd == -1)
			continue;
		close(fd);

		pexit_if(mount(from, to, "bind", MS_BIND, NULL) == -1,
		         "Mounting \"%s\" on \"%s\" failed", from, to);
	}
}

int main(int argc, char *argv[])
{
	const char *root, *cwd, *env, *exe;
	exit_if(argc < 5,
		"Usage: %s /path/to/root /work/directory /env/file /to/exec [args ...]", argv[0]);

	root = argv[1];
	cwd = argv[2];
	env = argv[3];
	exe = argv[4];

	load_env(env);

	setup_dev(root);

	pexit_if(chroot(root) == -1, "Chroot \"%s\" failed", root);
	pexit_if(chdir(cwd) == -1, "Chdir \"%s\" failed", cwd);

	/* XXX(vc): note that since execvp() is happening post-chroot, the
	 * app's environment settings correctly affect the PATH search.
	 * This is why execvpe() isn't being used, we manipulate the environment
	 * manually then let it potentially affect execvp().  execvpe() simply
	 * passes the environment to execve() _after_ performing the search, not
	 * what we want here. */
	pexit_if(execvp(exe, &argv[4]) == -1 &&
		 errno != ENOENT && errno != EACCES,
		 "Exec of \"%s\" failed", exe);
	diag(exe);

	return EXIT_FAILURE;
}
