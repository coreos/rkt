#ifndef _ARGS_H
#define _ARGS_H

#include <stdint.h>

typedef struct _args_t {
	const int	*blacklist;
	int		blacklist_len;
	uint64_t	caps_kept;
	uint64_t	caps_dropped;
	int		kept_fd;
	uint64_t	namespaces;
	int		no_pty;
	int		parent_isig;
	const char	*pid_file;
	int		quiet;
	const char	**target_argv;
	const char	*target_dir;
	const char	*target_name;
	const char	*target_uuid;
} args_t;

args_t * args_handle(int, const char *[]);

#endif
