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
#include <inttypes.h>
#include <sched.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/capability.h>
#include <unistd.h>

#include "args.h"
#include "fatal.h"
#include "mounts.h"

static args_t capsule_args = {
	.caps_kept = (
#define cap_keep(_sym, _desc) \
				(1ULL << _sym) |
#define cap(_sym, _desc)
#include "defs/capabilities.def"
	0ULL),
	.namespaces = (
#define ns_default(_sym, _desc)	_sym |
#define ns(_sym, _desc)
#include "defs/namespaces.def"
	0ULL),
	.kept_fd = -1,
};


/* map a capability name to its numeric constant via capabilities.def */
static uint64_t str_to_cap(const char *str)
{
#define cap(_sym, _desc)		\
	if(!strcasecmp(str, #_sym))	\
		return (1ULL << _sym);
#define cap_keep(_sym, _desc)		\
	if(!strcasecmp(str, #_sym))	\
		return (1ULL << _sym);
#include "defs/capabilities.def"

	exit_if(1, "unsupported capability: %s", str);
	return -1;
}

/* map a namespace name to its numeric constant via namespaces.def */
static uint64_t str_to_ns(const char *str)
{
#define ns(_sym, _desc)			\
	if(!strcasecmp(str, #_sym))	\
		return _sym;
#define ns_default(_sym, _desc)		\
	if(!strcasecmp(str, #_sym))	\
		return _sym;
#include "defs/namespaces.def"

	exit_if(1, "unsupported namespace: %s", str);
	return -1;
}

/* map a comma-separated list of symbolic names to set bit positions in a uint64_t */
static uint64_t str_to_bits(const char *str, uint64_t(*str_to_bit)(const char *))
{
	char		*dup, *cur, *sep;
	uint64_t	bits = 0;

	pexit_if((dup = strdup(str)) == NULL, "error duplicating str");

	cur = dup;
	do {
		sep = strchr(cur, ',');
		if(sep) {
			*sep = '\0';
		}
		bits |= str_to_bit(cur);
	} while(sep != NULL && (cur = (sep + 1)));

	free(dup);

	return bits;
}

/* argv parsing/handling */

#define ARGS_HALT	0
#define ARGS_CONTINUE	1
#define ARGS_EXIT	2

/* argument handlers, these are referenced by arguments.def and called when
 * matched by args_handle() */

/* help printers, generated union types are just to get a column width for formatting */
static int handle_help(args_t *ca, const char *arg)
{
	union _help_cw {
#define arg(_arg, _func, _desc)		char __##_func[sizeof(#_arg)];
#define arg_param(_arg, _func, _desc)	char __##_func[sizeof(#_arg) + sizeof("ARG")];
#include "defs/arguments.def"
	};

	puts("Capsule: containers for rocket.");
#define arg(_arg, _func, _desc)		\
	printf(" --%-*s  %s\n", (int)sizeof(union _help_cw), _arg, _desc);
#define arg_param(_arg, _func, _desc)	\
	printf(" --%-*s  %s\n", (int)sizeof(union _help_cw), _arg " ARG", _desc);
#include "defs/arguments.def"
	return ARGS_EXIT;
}

static int handle_blacklist_help(args_t *ca, const char *arg)
{
	union _bl_cw {
#define sys_bl(_sym, _desc) char __##_sym[sizeof(#_sym)];
#include "defs/blacklist.def"
	};

	puts("Default system-call blacklist:");
#define sys_bl(_sym, _desc)	\
	printf(" %-*s\t%s\n", (int)sizeof(union _bl_cw), #_sym, _desc);
#include "defs/blacklist.def"
	return ARGS_EXIT;
}

static int handle_caps_help(args_t *ca, const char *arg)
{
	union _caps_cw {
#define cap_keep(_sym, _desc)	char __##_sym[sizeof(#_sym)];
#define cap(_sym, _desc)	char __##_sym[sizeof(#_sym)];
#include "defs/capabilities.def"
	};

	puts("Capabilities (- indicates dropped by default):");
#define cap_keep(_sym, _desc)		\
	printf("  %-*s  %s\n", (int)sizeof(union _caps_cw), #_sym, _desc);
#define cap(_sym, _desc)		\
	printf("- %-*s  %s\n", (int)sizeof(union _caps_cw), #_sym, _desc);
#include "defs/capabilities.def"
	return ARGS_EXIT;
}

static int handle_ns_help(args_t *ca, const char *arg)
{
	union _ns_cw {
#define ns_default(_sym, _desc)	char __##_sym[sizeof(#_sym)];
#define ns(_sym, _desc)		char __##_sym[sizeof(#_sym)];
#include "defs/namespaces.def"
	};

	puts("Namespaces (- indicates off by default):");
#define ns_default(_sym, _desc)	\
	printf("  %-*s  %s\n", (int)sizeof(union _ns_cw), #_sym, _desc);
#define ns(_sym, _desc)		\
	printf("- %-*s  %s\n", (int)sizeof(union _ns_cw), #_sym, _desc);
#include "defs/namespaces.def"
	return ARGS_EXIT;
}

#define handle_once(_what, _test, _assign)			\
	exit_if(_test, "%s must occur only once", _what);	\
	_assign;

/* accumulate list of bind mounts */
static int handle_bind(args_t *ca, const char *arg, const char **param)
{
	mounts_add_bind(*param, 0);
	return ARGS_CONTINUE;
}

/* accumulate list of bind mounts */
static int handle_bind_ro(args_t *ca, const char *arg, const char **param)
{
	mounts_add_bind(*param, 1);
	return ARGS_CONTINUE;
}

/* accumulate list of tmpfs mounts */
static int handle_tmpfs(args_t *ca, const char *arg, const char **param)
{
	mounts_add_tmpfs(*param);
	return ARGS_CONTINUE;
}

/* manipulate capabilities set */
static int handle_caps(args_t *ca, const char *arg, const char **param)
{
	ca->caps_kept |= str_to_bits(*param, str_to_cap);
	return ARGS_CONTINUE;
}

/* manipulate capabilities set */
static int handle_drop_caps(args_t *ca, const char *arg, const char **param)
{
	ca->caps_dropped |= str_to_bits(*param, str_to_cap);
	return ARGS_CONTINUE;
}

/* set namespaces to create */
static int handle_namespaces(args_t *ca, const char *arg, const char **param)
{
	static int set;

	handle_once(arg, set, set = 1);
	ca->namespaces = str_to_bits(*param, str_to_ns);
	return ARGS_CONTINUE;
}

/* set syscall blacklist */
static int handle_blacklist(args_t *ca, const char *arg, const char **param)
{
	/* TODO(vc): convert comma-separated list of syscalls into list of numbers */
	puts("TODO: --blacklist unimplemented");
	return ARGS_EXIT;
}

/* target directory to use as root */
static int handle_directory(args_t *ca, const char *arg, const char **param)
{
	char *tgt;

	pexit_if((tgt = canonicalize_file_name(*param)) == NULL,
		 "failed to canonicalize \"%s\"", *param);
	exit_if(!strcmp(tgt, "/"),
		"using \"/\" for --directory unsupported");
	handle_once(arg, ca->target_dir, ca->target_dir = tgt);
	return ARGS_CONTINUE;
}

/* fd to keep open */
static int handle_keep_fd(args_t *ca, const char *arg, const char **param)
{
	handle_once(arg, -1 != ca->kept_fd, ca->kept_fd = atoi(*param));
	return ARGS_CONTINUE;
}

/* pid file to write */
static int handle_pidfile(args_t *ca, const char *arg, const char **param)
{
	handle_once(arg, ca->pid_file, ca->pid_file = *param);
	return ARGS_CONTINUE;
}

/* target command + args to run */
static int handle_cmd(args_t *ca, const char *arg, const char **param)
{
	handle_once(arg, ca->target_argv, ca->target_argv = param);
	return ARGS_HALT;
}

/* target uuid */
static int handle_uuid(args_t *ca, const char *arg, const char **param)
{
	handle_once(arg, ca->target_uuid, ca->target_uuid = *param);
	return ARGS_CONTINUE;
}

/* quiet */
static int handle_quiet(args_t *ca, const char *arg)
{
	handle_once(arg, ca->quiet, ca->quiet = 1);
	return ARGS_CONTINUE;
}

/* hostname */
static int handle_hostname(args_t *ca, const char *arg, const char **param)
{
	handle_once(arg, ca->target_name, ca->target_name = *param);
	return ARGS_CONTINUE;
}

/* no pty? */
static int handle_nopty(args_t *ca, const char *arg)
{
	handle_once(arg, ca->no_pty, ca->no_pty = 1);
	return ARGS_CONTINUE;
}

/* keep ISIG bit set in parent tty? (see termios(3)) */
static int handle_parent_isig(args_t *ca, const char *arg)
{
	handle_once(arg, ca->parent_isig, ca->parent_isig = 1);
	return ARGS_CONTINUE;
}

/* args_handle() scans argv comparing against arguments.def and calls matching functions */
args_t * args_handle(int argc, const char *argv[])
{
	int	i;
	args_t	*ca = &capsule_args;

	for(i = 1; i < argc; i++) {
		const char *arg = argv[i];

		if(arg[0] == '-' && arg[1] == '-') {
#define arg(_arg, _func, _desc)					\
			if(!strcmp(&arg[2], _arg)) {		\
				switch(_func(ca, arg)) {	\
				case ARGS_EXIT:			\
					exit(0);		\
				case ARGS_HALT:			\
					goto _halt;		\
				case ARGS_CONTINUE:		\
					continue;		\
				}				\
			}

#define arg_param(_arg, _func, _desc)						\
			if(!strcmp(&arg[2], _arg)) {				\
				exit_if(argc - i < 2,				\
					"--" _arg " requires a paramater")	\
				switch(_func(ca, arg, &argv[i + 1])) {		\
				case ARGS_EXIT:					\
					exit(0);				\
				case ARGS_HALT:					\
					goto _halt;				\
				case ARGS_CONTINUE:				\
					i++; 					\
					continue;				\
				}						\
			}
#include "defs/arguments.def"
		}

		exit_if(1, "invalid argument: \"%s\"", arg);
	}
_halt:

	exit_if(!ca->target_dir, "--directory required");
	exit_if(!ca->target_argv, "-- CMD [ARGS] required");

	if(!ca->target_name) {
		ca->target_name = basename(ca->target_dir);
	}

	return ca;
}

