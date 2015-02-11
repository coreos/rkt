#ifndef _FATAL_H
#define _FATAL_H

#include <errno.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

extern int exit_err;

#define exit_if(_cond, _fmt, _args...)				\
	exit_err++;						\
	if(_cond) {						\
		fprintf(stderr, "Error: " _fmt "\n", ##_args);	\
		exit(exit_err);					\
	}
#define pexit_if(_cond, _fmt, _args...)				\
	exit_if(_cond, _fmt ": %s", ##_args, strerror(errno))

#endif
