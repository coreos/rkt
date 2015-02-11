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
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/mount.h>
#include <sys/stat.h>
#include <sys/types.h>
#include <unistd.h>

#include "fatal.h"

typedef enum mount_type_t {
	MOUNT_BIND,
	MOUNT_TMPFS
} mount_type_t;

typedef struct _any_t {
	const char	*container_path;
} any_t;

typedef struct _bind_t {
	const char	*container_path;
	const char	*host_path;
	int		readonly;
} bind_t;

typedef struct _tmpfs_t {
	const char	*container_path;
	const char	*options;
} tmpfs_t;

typedef struct _mount_t {
	mount_type_t	type;
	union {
		any_t	any;
		bind_t	bind;
		tmpfs_t	tmpfs;
	};
} mount_t;

static mount_t	*mounts;
static int	n_mounts;

/* add a new uninitialized mount to the mounts array and return it's typed member */
static void * new_mount(mount_type_t t)
{
	mount_t	*m;
	n_mounts++;
	pexit_if((mounts = realloc(mounts, sizeof(mount_t) * n_mounts)) == NULL,
		 "error enlarging mounts to %i", n_mounts);
	m = &mounts[n_mounts - 1];
	memset(m, 0, sizeof(*m));
	m->type = t;
	switch(t) {
		case MOUNT_BIND:
			return &m->bind;
		case MOUNT_TMPFS:
			return &m->tmpfs;
	}
	return NULL;
}

/* split a string on fs (for "foo:bar" style mount strings) */
static void dup_trysplit(const char *str, const char **a, const char **b, char fs)
{
	char	*dup, *sep;

	pexit_if((dup = strdup(str)) == NULL,
		 "error duplicating str");
	*a = dup;
	*b = NULL;
	if((sep = strchr(dup, fs)) != NULL) {
		*sep = '\0';
		*b = sep + 1;
	}
}

/* transform a bind string to a mount_t */
static void str_to_bind(const char *str, int ro)
{
	bind_t	*bind = new_mount(MOUNT_BIND);

	dup_trysplit(str, &bind->host_path, &bind->container_path, ':');
	if(!bind->container_path)
		bind->container_path = bind->host_path;

	bind->readonly = ro;

	exit_if(bind->host_path[0] != '/' || bind->container_path[0] != '/',
		 "bind paths must be absolute: %s -> %s",
		 bind->host_path, bind->container_path);
}

/* transform a tmpfs mount string to a mount_t */
static void str_to_tmpfs(const char *str)
{
	tmpfs_t	*tmpfs = new_mount(MOUNT_TMPFS);

	dup_trysplit(str, &tmpfs->container_path, &tmpfs->options, ':');

	exit_if(tmpfs->container_path[0] != '/',
		"tmpfs path must be absolute: %s",
		tmpfs->container_path);
}

/* add a bind mount of the format "host:container", or just "host" */
void mounts_add_bind(const char *str, int ro)
{
	str_to_bind(str, ro);
}

/* add a tmpfs mount of the format "container:options", or just "container" */
void mounts_add_tmpfs(const char *str)
{
	str_to_tmpfs(str);
}

/* mkdir -p, using specified mode for each dir created */
static void mkdir_parents(const char *path, mode_t mode)
{
	char	*dup, *cur, *next;
	int	len;

/* TODO(vc): make more robust, trusts inputs to not do things like /foo/bar/../../escape/contdir */
	pexit_if((dup = strdup(path)) == NULL,
		 "unable to duplicate path");

	/* strip all contiguous trailing '/'s */
	len = strlen(path) + 1;
	while(--len > 0 && dup[len] == '/');
	dup[len] = '\0';

	if(len == 0)
		return;

	/* try mkdir every name along the path except the last */
	cur = dup + 1;
	while((next = strchr(cur, '/')) != NULL) {
		*next = '\0';
		pexit_if(mkdir(dup, mode) == -1 && errno != EEXIST,
			 "unable to mkdir \"%s\"", dup);
		*next = '/';
		cur = next + 1;
	}

	free(dup);
}

/* mount a mount */
static void mount_mount(const char *target_dir, mount_t *m)
{
	struct stat	src_st, tgt_st;
	char		*mnt_tgt;
	int		ret;
	const char	*mnt_src = NULL, *mnt_type = NULL;
	unsigned long	mnt_flags = 0;
	const void	*mnt_data = NULL;

	if(m->type == MOUNT_BIND) {
		pexit_if(lstat(m->bind.host_path, &src_st) == -1,
			 "bind path \"%s\" inaccessible in host",
			 m->bind.host_path);
	}

	pexit_if((asprintf(&mnt_tgt, "%s%s", target_dir,
			   m->any.container_path)) == -1,
		 "error allocating container path");
	pexit_if((ret = lstat(mnt_tgt, &tgt_st)) == -1 &&
		 errno != ENOENT,
		 "unable to stat \"%s\" in container", mnt_tgt);

	if(ret == 0) {
		exit_if(m->type == MOUNT_BIND &&
			(src_st.st_mode & S_IFMT) !=
			(tgt_st.st_mode & S_IFMT),
			"bind mount host and container path types differ");
	} else if(errno == ENOENT) {
		mkdir_parents(mnt_tgt, 0755);

		if(m->type == MOUNT_BIND && !S_ISDIR(src_st.st_mode)) {
			if(S_ISREG(src_st.st_mode)) {
				int fd;

				pexit_if((fd = creat(mnt_tgt, 0640)) == -1,
					 "error creating regular file \"%s\"",
					 mnt_tgt);
				pexit_if(close(fd) == -1, "error closing fd");
			} else if(S_ISFIFO(src_st.st_mode)) {
				pexit_if(mkfifo(mnt_tgt, 0640) == -1,
					 "error creating fifo @ \"%s\"",
					 mnt_tgt);
			} else if(S_ISSOCK(src_st.st_mode)) {
				pexit_if(mknod(mnt_tgt, 0640|S_IFSOCK, 0) == -1,
					 "error creating socket @ \"%s\"",
					 mnt_tgt);
			} else {
				exit_if(1, "unsupported bind type at \"%s\"",
					m->bind.host_path);
			}
		} else {
			pexit_if(mkdir(mnt_tgt, 0755) == -1,
				 "unable to create directory \"%s\"",
				 mnt_tgt);
		}
	}

	switch(m->type) {
	case MOUNT_BIND:
		mnt_src = m->bind.host_path;
		mnt_flags = MS_BIND;
		break;
	case MOUNT_TMPFS:
		mnt_type = "tmpfs";
		mnt_flags = MS_NODEV|MS_STRICTATIME;
		mnt_data = m->tmpfs.options;
		break;
	}
	pexit_if(mount(mnt_src, mnt_tgt, mnt_type, mnt_flags, mnt_data) == -1,
		 "unable to mount at \"%s\"", mnt_tgt);

	/* TODO(vc): honor m->bind.readonly recursively like nspawn */

	free(mnt_tgt);
}

/* mount the supplied array of mounts under target_dir */
void mounts_mount_all(const char *target_dir)
{
	int i;

	for(i = 0; i < n_mounts; i++) {
		mount_mount(target_dir, &mounts[i]);
	}
}
