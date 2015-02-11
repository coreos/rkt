#ifndef _MOUNTS_H
#define _MOUNTS_H

void mounts_add_bind(const char *, int);
void mounts_add_tmpfs(const char *);
void mounts_mount_all(const char *);

#endif
