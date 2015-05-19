// Copyright 2014-2015 CoreOS, Inc.
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
#include <sys/prctl.h>


static int errornum;
#define exit_if(_cond, _fmt, _args...)				\
	errornum++;						\
	if(_cond) {						\
		fprintf(stderr, _fmt "\n", ##_args);		\
		exit(errornum);					\
	}
#define pexit_if(_cond, _fmt, _args...)				\
	exit_if(_cond, _fmt ": %s", ##_args, strerror(errno))

int main(int argc, char *argv[])
{
	FILE *f;
	int sockfd;
	char *pidfile;
	char buffer[4096];
	char *ptr;
	int r;
	char *needle = "X_NSPAWN_LEADER_PID=";
	int pid;

	exit_if(argc < 3,
		"Usage: %s sockfd pidfile", argv[0])

	sockfd = atoi(argv[1]);
	pidfile = argv[2];

	r = prctl(PR_SET_PDEATHSIG, SIGKILL);
	pexit_if (r < 0, "Cannot set PR_SET_PDEATHSIG");

	r = read(sockfd, buffer, sizeof(buffer));
	pexit_if (r <= 0, "Cannot read socket file");

	f = fopen(pidfile, "we");
	pexit_if(!f, "Cannot write to file '%s'", pidfile);

	ptr = strstr(buffer, needle);
	exit_if(ptr == NULL, "Cannot find %s", needle);
	ptr += strlen(needle);
	pid = atoi(ptr);

	fprintf(f, "%d\n", pid);

	fflush(f);
	fclose(f);

	return EXIT_SUCCESS;
}
