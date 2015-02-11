#ifndef _PTY_H
#define _PTY_H

void pty_alloc(int *, char **);
void pty_begin_session(const char *);
void pty_manage(int, int, volatile int *);

#endif
