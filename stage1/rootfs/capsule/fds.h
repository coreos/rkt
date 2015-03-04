#ifndef _FDS_H
#define _FDS_H

int fds_get_listen_fds(void);
void fds_close_most(int, int);
void fds_close_rest(int);

#endif
