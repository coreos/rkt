/**
 * Enhanced Seccomp ARM Syscall Table
 *
 * Copyright (c) 2013 Red Hat <pmoore@redhat.com>
 * Author: Paul Moore <pmoore@redhat.com>
 */

/*
 * This library is free software; you can redistribute it and/or modify it
 * under the terms of version 2.1 of the GNU Lesser General Public License as
 * published by the Free Software Foundation.
 *
 * This library is distributed in the hope that it will be useful, but WITHOUT
 * ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or
 * FITNESS FOR A PARTICULAR PURPOSE.  See the GNU Lesser General Public License
 * for more details.
 *
 * You should have received a copy of the GNU Lesser General Public License
 * along with this library; if not, see <http://www.gnu.org/licenses>.
 */

#include <string.h>

#include <seccomp.h>

#include "arch.h"
#include "arch-arm.h"

#define __NR_OABI_SYSCALL_BASE	0x900000

/* NOTE: we currently only support the ARM EABI, more info at the URL below:
 *       -> http://wiki.embeddedarm.com/wiki/EABI_vs_OABI */
#if 1
#define __NR_SYSCALL_BASE	0
#else
#define __NR_SYSCALL_BASE	__NR_OABI_SYSCALL_BASE
#endif

/* NOTE: based on Linux 3.19 */
const struct arch_syscall_def arm_syscall_table[] = { \
	/* NOTE: arm_sync_file_range() and sync_file_range2() share values */
	{ "_llseek", (__NR_SYSCALL_BASE + 140) },
	{ "_newselect", (__NR_SYSCALL_BASE + 142) },
	{ "_sysctl", (__NR_SYSCALL_BASE + 149) },
	{ "accept", (__NR_SYSCALL_BASE + 285) },
	{ "accept4", (__NR_SYSCALL_BASE + 366) },
	{ "access", (__NR_SYSCALL_BASE + 33) },
	{ "acct", (__NR_SYSCALL_BASE + 51) },
	{ "add_key", (__NR_SYSCALL_BASE + 309) },
	{ "adjtimex", (__NR_SYSCALL_BASE + 124) },
	{ "afs_syscall", __PNR_afs_syscall },
	{ "alarm", (__NR_SYSCALL_BASE + 27) },
	{ "arm_fadvise64_64", (__NR_SYSCALL_BASE + 270) },
	{ "arm_sync_file_range", (__NR_SYSCALL_BASE + 341) },
	{ "arch_prctl", __PNR_arch_prctl },
	{ "bdflush", (__NR_SYSCALL_BASE + 134) },
	{ "bind", (__NR_SYSCALL_BASE + 282) },
	{ "bpf", (__NR_SYSCALL_BASE + 386) },
	{ "break", __PNR_break },
	{ "brk", (__NR_SYSCALL_BASE + 45) },
	{ "cachectl", __PNR_cachectl },
	{ "cacheflush", __PNR_cacheflush },
	{ "capget", (__NR_SYSCALL_BASE + 184) },
	{ "capset", (__NR_SYSCALL_BASE + 185) },
	{ "chdir", (__NR_SYSCALL_BASE + 12) },
	{ "chmod", (__NR_SYSCALL_BASE + 15) },
	{ "chown", (__NR_SYSCALL_BASE + 182) },
	{ "chown32", (__NR_SYSCALL_BASE + 212) },
	{ "chroot", (__NR_SYSCALL_BASE + 61) },
	{ "clock_adjtime", (__NR_SYSCALL_BASE + 372) },
	{ "clock_getres", (__NR_SYSCALL_BASE + 264) },
	{ "clock_gettime", (__NR_SYSCALL_BASE + 263) },
	{ "clock_nanosleep", (__NR_SYSCALL_BASE + 265) },
	{ "clock_settime", (__NR_SYSCALL_BASE + 262) },
	{ "clone", (__NR_SYSCALL_BASE + 120) },
	{ "close", (__NR_SYSCALL_BASE +  6) },
	{ "connect", (__NR_SYSCALL_BASE + 283) },
	{ "creat", (__NR_SYSCALL_BASE +  8) },
	{ "create_module", __PNR_create_module },
	{ "delete_module", (__NR_SYSCALL_BASE + 129) },
	{ "dup", (__NR_SYSCALL_BASE + 41) },
	{ "dup2", (__NR_SYSCALL_BASE + 63) },
	{ "dup3", (__NR_SYSCALL_BASE + 358) },
	{ "epoll_create", (__NR_SYSCALL_BASE + 250) },
	{ "epoll_create1", (__NR_SYSCALL_BASE + 357) },
	{ "epoll_ctl", (__NR_SYSCALL_BASE + 251) },
	{ "epoll_ctl_old", __PNR_epoll_ctl_old },
	{ "epoll_pwait", (__NR_SYSCALL_BASE + 346) },
	{ "epoll_wait", (__NR_SYSCALL_BASE + 252) },
	{ "epoll_wait_old", __PNR_epoll_wait_old },
	{ "eventfd", (__NR_SYSCALL_BASE + 351) },
	{ "eventfd2", (__NR_SYSCALL_BASE + 356) },
	{ "execve", (__NR_SYSCALL_BASE + 11) },
	{ "execveat", (__NR_SYSCALL_BASE + 387) },
	{ "exit", (__NR_SYSCALL_BASE +  1) },
	{ "exit_group", (__NR_SYSCALL_BASE + 248) },
	{ "faccessat", (__NR_SYSCALL_BASE + 334) },
	{ "fadvise64", __PNR_fadvise64 },
	{ "fadvise64_64", __PNR_fadvise64_64 },
	{ "fallocate", (__NR_SYSCALL_BASE + 352) },
	{ "fanotify_init", (__NR_SYSCALL_BASE + 367) },
	{ "fanotify_mark", (__NR_SYSCALL_BASE + 368) },
	{ "fchdir", (__NR_SYSCALL_BASE + 133) },
	{ "fchmod", (__NR_SYSCALL_BASE + 94) },
	{ "fchmodat", (__NR_SYSCALL_BASE + 333) },
	{ "fchown", (__NR_SYSCALL_BASE + 95) },
	{ "fchown32", (__NR_SYSCALL_BASE + 207) },
	{ "fchownat", (__NR_SYSCALL_BASE + 325) },
	{ "fcntl", (__NR_SYSCALL_BASE + 55) },
	{ "fcntl64", (__NR_SYSCALL_BASE + 221) },
	{ "fdatasync", (__NR_SYSCALL_BASE + 148) },
	{ "fgetxattr", (__NR_SYSCALL_BASE + 231) },
	{ "finit_module", (__NR_SYSCALL_BASE + 379) },
	{ "flistxattr", (__NR_SYSCALL_BASE + 234) },
	{ "flock", (__NR_SYSCALL_BASE + 143) },
	{ "fork", (__NR_SYSCALL_BASE +  2) },
	{ "fremovexattr", (__NR_SYSCALL_BASE + 237) },
	{ "fsetxattr", (__NR_SYSCALL_BASE + 228) },
	{ "fstat", (__NR_SYSCALL_BASE + 108) },
	{ "fstat64", (__NR_SYSCALL_BASE + 197) },
	{ "fstatat64", (__NR_SYSCALL_BASE + 327) },
	{ "fstatfs", (__NR_SYSCALL_BASE + 100) },
	{ "fstatfs64", (__NR_SYSCALL_BASE + 267) },
	{ "fsync", (__NR_SYSCALL_BASE + 118) },
	{ "ftime", __PNR_ftime },
	{ "ftruncate", (__NR_SYSCALL_BASE + 93) },
	{ "ftruncate64", (__NR_SYSCALL_BASE + 194) },
	{ "futex", (__NR_SYSCALL_BASE + 240) },
	{ "futimesat", (__NR_SYSCALL_BASE + 326) },
	{ "get_kernel_syms", __PNR_get_kernel_syms },
	{ "get_mempolicy", (__NR_SYSCALL_BASE + 320) },
	{ "get_robust_list", (__NR_SYSCALL_BASE + 339) },
	{ "get_thread_area", __PNR_get_thread_area },
	{ "getcpu", (__NR_SYSCALL_BASE + 345) },
	{ "getcwd", (__NR_SYSCALL_BASE + 183) },
	{ "getdents", (__NR_SYSCALL_BASE + 141) },
	{ "getdents64", (__NR_SYSCALL_BASE + 217) },
	{ "getegid", (__NR_SYSCALL_BASE + 50) },
	{ "getegid32", (__NR_SYSCALL_BASE + 202) },
	{ "geteuid", (__NR_SYSCALL_BASE + 49) },
	{ "geteuid32", (__NR_SYSCALL_BASE + 201) },
	{ "getgid", (__NR_SYSCALL_BASE + 47) },
	{ "getgid32", (__NR_SYSCALL_BASE + 200) },
	{ "getgroups", (__NR_SYSCALL_BASE + 80) },
	{ "getgroups32", (__NR_SYSCALL_BASE + 205) },
	{ "getitimer", (__NR_SYSCALL_BASE + 105) },
	{ "getpeername", (__NR_SYSCALL_BASE + 287) },
	{ "getpgid", (__NR_SYSCALL_BASE + 132) },
	{ "getpgrp", (__NR_SYSCALL_BASE + 65) },
	{ "getpid", (__NR_SYSCALL_BASE + 20) },
	{ "getpmsg", __PNR_getpmsg },
	{ "getppid", (__NR_SYSCALL_BASE + 64) },
	{ "getpriority", (__NR_SYSCALL_BASE + 96) },
	{ "getrandom", (__NR_SYSCALL_BASE + 384) },
	{ "getresgid", (__NR_SYSCALL_BASE + 171) },
	{ "getresgid32", (__NR_SYSCALL_BASE + 211) },
	{ "getresuid", (__NR_SYSCALL_BASE + 165) },
	{ "getresuid32", (__NR_SYSCALL_BASE + 209) },
	{ "getrlimit", (__NR_SYSCALL_BASE + 76) },
	{ "getrusage", (__NR_SYSCALL_BASE + 77) },
	{ "getsid", (__NR_SYSCALL_BASE + 147) },
	{ "getsockname", (__NR_SYSCALL_BASE + 286) },
	{ "getsockopt", (__NR_SYSCALL_BASE + 295) },
	{ "gettid", (__NR_SYSCALL_BASE + 224) },
	{ "gettimeofday", (__NR_SYSCALL_BASE + 78) },
	{ "getuid", (__NR_SYSCALL_BASE + 24) },
	{ "getuid32", (__NR_SYSCALL_BASE + 199) },
	{ "getxattr", (__NR_SYSCALL_BASE + 229) },
	{ "gtty", __PNR_gtty },
	{ "idle", __PNR_idle },
	{ "init_module", (__NR_SYSCALL_BASE + 128) },
	{ "inotify_add_watch", (__NR_SYSCALL_BASE + 317) },
	{ "inotify_init", (__NR_SYSCALL_BASE + 316) },
	{ "inotify_init1", (__NR_SYSCALL_BASE + 360) },
	{ "inotify_rm_watch", (__NR_SYSCALL_BASE + 318) },
	{ "io_cancel", (__NR_SYSCALL_BASE + 247) },
	{ "io_destroy", (__NR_SYSCALL_BASE + 244) },
	{ "io_getevents", (__NR_SYSCALL_BASE + 245) },
	{ "io_setup", (__NR_SYSCALL_BASE + 243) },
	{ "io_submit", (__NR_SYSCALL_BASE + 246) },
	{ "ioctl", (__NR_SYSCALL_BASE + 54) },
	{ "ioperm", __PNR_ioperm },
	{ "iopl", __PNR_iopl },
	{ "ioprio_get", (__NR_SYSCALL_BASE + 315) },
	{ "ioprio_set", (__NR_SYSCALL_BASE + 314) },
	{ "ipc", (__NR_SYSCALL_BASE + 117) },
	{ "kcmp", (__NR_SYSCALL_BASE + 378) },
	{ "kexec_file_load", __PNR_kexec_file_load },
	{ "kexec_load", (__NR_SYSCALL_BASE + 347) },
	{ "keyctl", (__NR_SYSCALL_BASE + 311) },
	{ "kill", (__NR_SYSCALL_BASE + 37) },
	{ "lchown", (__NR_SYSCALL_BASE + 16) },
	{ "lchown32", (__NR_SYSCALL_BASE + 198) },
	{ "lgetxattr", (__NR_SYSCALL_BASE + 230) },
	{ "link", (__NR_SYSCALL_BASE +  9) },
	{ "linkat", (__NR_SYSCALL_BASE + 330) },
	{ "listen", (__NR_SYSCALL_BASE + 284) },
	{ "listxattr", (__NR_SYSCALL_BASE + 232) },
	{ "llistxattr", (__NR_SYSCALL_BASE + 233) },
	{ "lock", __PNR_lock },
	{ "lookup_dcookie", (__NR_SYSCALL_BASE + 249) },
	{ "lremovexattr", (__NR_SYSCALL_BASE + 236) },
	{ "lseek", (__NR_SYSCALL_BASE + 19) },
	{ "lsetxattr", (__NR_SYSCALL_BASE + 227) },
	{ "lstat", (__NR_SYSCALL_BASE + 107) },
	{ "lstat64", (__NR_SYSCALL_BASE + 196) },
	{ "madvise", (__NR_SYSCALL_BASE + 220) },
	{ "mbind", (__NR_SYSCALL_BASE + 319) },
	{ "memfd_create", (__NR_SYSCALL_BASE + 385) },
	{ "migrate_pages", __PNR_migrate_pages },
	{ "mincore", (__NR_SYSCALL_BASE + 219) },
	{ "mkdir", (__NR_SYSCALL_BASE + 39) },
	{ "mkdirat", (__NR_SYSCALL_BASE + 323) },
	{ "mknod", (__NR_SYSCALL_BASE + 14) },
	{ "mknodat", (__NR_SYSCALL_BASE + 324) },
	{ "mlock", (__NR_SYSCALL_BASE + 150) },
	{ "mlockall", (__NR_SYSCALL_BASE + 152) },
	{ "mmap", (__NR_SYSCALL_BASE + 90) },
	{ "mmap2", (__NR_SYSCALL_BASE + 192) },
	{ "modify_ldt", __PNR_modify_ldt },
	{ "mount", (__NR_SYSCALL_BASE + 21) },
	{ "move_pages", (__NR_SYSCALL_BASE + 344) },
	{ "mprotect", (__NR_SYSCALL_BASE + 125) },
	{ "mpx", __PNR_mpx },
	{ "mq_getsetattr", (__NR_SYSCALL_BASE + 279) },
	{ "mq_notify", (__NR_SYSCALL_BASE + 278) },
	{ "mq_open", (__NR_SYSCALL_BASE + 274) },
	{ "mq_timedreceive", (__NR_SYSCALL_BASE + 277) },
	{ "mq_timedsend", (__NR_SYSCALL_BASE + 276) },
	{ "mq_unlink", (__NR_SYSCALL_BASE + 275) },
	{ "mremap", (__NR_SYSCALL_BASE + 163) },
	{ "msgctl", (__NR_SYSCALL_BASE + 304) },
	{ "msgget", (__NR_SYSCALL_BASE + 303) },
	{ "msgrcv", (__NR_SYSCALL_BASE + 302) },
	{ "msgsnd", (__NR_SYSCALL_BASE + 301) },
	{ "msync", (__NR_SYSCALL_BASE + 144) },
	{ "munlock", (__NR_SYSCALL_BASE + 151) },
	{ "munlockall", (__NR_SYSCALL_BASE + 153) },
	{ "munmap", (__NR_SYSCALL_BASE + 91) },
	{ "name_to_handle_at", (__NR_SYSCALL_BASE + 370) },
	{ "nanosleep", (__NR_SYSCALL_BASE + 162) },
	{ "newfstatat", __PNR_newfstatat },
	{ "nfsservctl", (__NR_SYSCALL_BASE + 169) },
	{ "nice", (__NR_SYSCALL_BASE + 34) },
	{ "oldfstat", __PNR_oldfstat },
	{ "oldlstat", __PNR_oldlstat },
	{ "oldolduname", __PNR_oldolduname },
	{ "oldstat", __PNR_oldstat },
	{ "olduname", __PNR_olduname },
	{ "oldwait4", __PNR_oldwait4 },
	{ "open", (__NR_SYSCALL_BASE +  5) },
	{ "open_by_handle_at", (__NR_SYSCALL_BASE + 371) },
	{ "openat", (__NR_SYSCALL_BASE + 322) },
	{ "pause", (__NR_SYSCALL_BASE + 29) },
	{ "pciconfig_iobase", (__NR_SYSCALL_BASE + 271) },
	{ "pciconfig_read", (__NR_SYSCALL_BASE + 272) },
	{ "pciconfig_write", (__NR_SYSCALL_BASE + 273) },
	{ "perf_event_open", (__NR_SYSCALL_BASE + 364) },
	{ "personality", (__NR_SYSCALL_BASE + 136) },
	{ "pipe", (__NR_SYSCALL_BASE + 42) },
	{ "pipe2", (__NR_SYSCALL_BASE + 359) },
	{ "pivot_root", (__NR_SYSCALL_BASE + 218) },
	{ "poll", (__NR_SYSCALL_BASE + 168) },
	{ "ppoll", (__NR_SYSCALL_BASE + 336) },
	{ "prctl", (__NR_SYSCALL_BASE + 172) },
	{ "pread64", (__NR_SYSCALL_BASE + 180) },
	{ "preadv", (__NR_SYSCALL_BASE + 361) },
	{ "prlimit64", (__NR_SYSCALL_BASE + 369) },
	{ "process_vm_readv", (__NR_SYSCALL_BASE + 376) },
	{ "process_vm_writev", (__NR_SYSCALL_BASE + 377) },
	{ "prof", __PNR_prof },
	{ "profil", __PNR_profil },
	{ "pselect6", (__NR_SYSCALL_BASE + 335) },
	{ "ptrace", (__NR_SYSCALL_BASE + 26) },
	{ "putpmsg", __PNR_putpmsg },
	{ "pwrite64", (__NR_SYSCALL_BASE + 181) },
	{ "pwritev", (__NR_SYSCALL_BASE + 362) },
	{ "query_module", __PNR_query_module },
	{ "quotactl", (__NR_SYSCALL_BASE + 131) },
	{ "read", (__NR_SYSCALL_BASE +  3) },
	{ "readahead", (__NR_SYSCALL_BASE + 225) },
	{ "readdir", (__NR_SYSCALL_BASE + 89) },
	{ "readlink", (__NR_SYSCALL_BASE + 85) },
	{ "readlinkat", (__NR_SYSCALL_BASE + 332) },
	{ "readv", (__NR_SYSCALL_BASE + 145) },
	{ "reboot", (__NR_SYSCALL_BASE + 88) },
	{ "recv", (__NR_SYSCALL_BASE + 291) },
	{ "recvfrom", (__NR_SYSCALL_BASE + 292) },
	{ "recvmmsg", (__NR_SYSCALL_BASE + 365) },
	{ "recvmsg", (__NR_SYSCALL_BASE + 297) },
	{ "remap_file_pages", (__NR_SYSCALL_BASE + 253) },
	{ "removexattr", (__NR_SYSCALL_BASE + 235) },
	{ "rename", (__NR_SYSCALL_BASE + 38) },
	{ "renameat", (__NR_SYSCALL_BASE + 329) },
	{ "renameat2", (__NR_SYSCALL_BASE + 382) },
	{ "request_key", (__NR_SYSCALL_BASE + 310) },
	{ "restart_syscall", (__NR_SYSCALL_BASE +  0) },
	{ "rmdir", (__NR_SYSCALL_BASE + 40) },
	{ "rt_sigaction", (__NR_SYSCALL_BASE + 174) },
	{ "rt_sigpending", (__NR_SYSCALL_BASE + 176) },
	{ "rt_sigprocmask", (__NR_SYSCALL_BASE + 175) },
	{ "rt_sigqueueinfo", (__NR_SYSCALL_BASE + 178) },
	{ "rt_sigreturn", (__NR_SYSCALL_BASE + 173) },
	{ "rt_sigsuspend", (__NR_SYSCALL_BASE + 179) },
	{ "rt_sigtimedwait", (__NR_SYSCALL_BASE + 177) },
	{ "rt_tgsigqueueinfo", (__NR_SYSCALL_BASE + 363) },
	{ "sched_get_priority_max", (__NR_SYSCALL_BASE + 159) },
	{ "sched_get_priority_min", (__NR_SYSCALL_BASE + 160) },
	{ "sched_getaffinity", (__NR_SYSCALL_BASE + 242) },
	{ "sched_getattr", (__NR_SYSCALL_BASE + 381) },
	{ "sched_getparam", (__NR_SYSCALL_BASE + 155) },
	{ "sched_getscheduler", (__NR_SYSCALL_BASE + 157) },
	{ "sched_rr_get_interval", (__NR_SYSCALL_BASE + 161) },
	{ "sched_setaffinity", (__NR_SYSCALL_BASE + 241) },
	{ "sched_setattr", (__NR_SYSCALL_BASE + 380) },
	{ "sched_setparam", (__NR_SYSCALL_BASE + 154) },
	{ "sched_setscheduler", (__NR_SYSCALL_BASE + 156) },
	{ "sched_yield", (__NR_SYSCALL_BASE + 158) },
	{ "seccomp", (__NR_SYSCALL_BASE + 383) },
	{ "security", __PNR_security },
	{ "select", (__NR_SYSCALL_BASE + 82) },
	{ "semctl", (__NR_SYSCALL_BASE + 300) },
	{ "semget", (__NR_SYSCALL_BASE + 299) },
	{ "semop", (__NR_SYSCALL_BASE + 298) },
	{ "semtimedop", (__NR_SYSCALL_BASE + 312) },
	{ "send", (__NR_SYSCALL_BASE + 289) },
	{ "sendfile", (__NR_SYSCALL_BASE + 187) },
	{ "sendfile64", (__NR_SYSCALL_BASE + 239) },
	{ "sendmmsg", (__NR_SYSCALL_BASE + 374) },
	{ "sendmsg", (__NR_SYSCALL_BASE + 296) },
	{ "sendto", (__NR_SYSCALL_BASE + 290) },
	{ "set_mempolicy", (__NR_SYSCALL_BASE + 321) },
	{ "set_robust_list", (__NR_SYSCALL_BASE + 338) },
	{ "set_thread_area", __PNR_set_thread_area },
	{ "set_tid_address", (__NR_SYSCALL_BASE + 256) },
	{ "setdomainname", (__NR_SYSCALL_BASE + 121) },
	{ "setfsgid", (__NR_SYSCALL_BASE + 139) },
	{ "setfsgid32", (__NR_SYSCALL_BASE + 216) },
	{ "setfsuid", (__NR_SYSCALL_BASE + 138) },
	{ "setfsuid32", (__NR_SYSCALL_BASE + 215) },
	{ "setgid", (__NR_SYSCALL_BASE + 46) },
	{ "setgid32", (__NR_SYSCALL_BASE + 214) },
	{ "setgroups", (__NR_SYSCALL_BASE + 81) },
	{ "setgroups32", (__NR_SYSCALL_BASE + 206) },
	{ "sethostname", (__NR_SYSCALL_BASE + 74) },
	{ "setitimer", (__NR_SYSCALL_BASE + 104) },
	{ "setns", (__NR_SYSCALL_BASE + 375) },
	{ "setpgid", (__NR_SYSCALL_BASE + 57) },
	{ "setpriority", (__NR_SYSCALL_BASE + 97) },
	{ "setregid", (__NR_SYSCALL_BASE + 71) },
	{ "setregid32", (__NR_SYSCALL_BASE + 204) },
	{ "setresgid", (__NR_SYSCALL_BASE + 170) },
	{ "setresgid32", (__NR_SYSCALL_BASE + 210) },
	{ "setresuid", (__NR_SYSCALL_BASE + 164) },
	{ "setresuid32", (__NR_SYSCALL_BASE + 208) },
	{ "setreuid", (__NR_SYSCALL_BASE + 70) },
	{ "setreuid32", (__NR_SYSCALL_BASE + 203) },
	{ "setrlimit", (__NR_SYSCALL_BASE + 75) },
	{ "setsid", (__NR_SYSCALL_BASE + 66) },
	{ "setsockopt", (__NR_SYSCALL_BASE + 294) },
	{ "settimeofday", (__NR_SYSCALL_BASE + 79) },
	{ "setuid", (__NR_SYSCALL_BASE + 23) },
	{ "setuid32", (__NR_SYSCALL_BASE + 213) },
	{ "setxattr", (__NR_SYSCALL_BASE + 226) },
	{ "sgetmask", __PNR_sgetmask },
	{ "shmat", (__NR_SYSCALL_BASE + 305) },
	{ "shmctl", (__NR_SYSCALL_BASE + 308) },
	{ "shmdt", (__NR_SYSCALL_BASE + 306) },
	{ "shmget", (__NR_SYSCALL_BASE + 307) },
	{ "shutdown", (__NR_SYSCALL_BASE + 293) },
	{ "sigaction", (__NR_SYSCALL_BASE + 67) },
	{ "sigaltstack", (__NR_SYSCALL_BASE + 186) },
	{ "signal", __PNR_signal },
	{ "signalfd", (__NR_SYSCALL_BASE + 349) },
	{ "signalfd4", (__NR_SYSCALL_BASE + 355) },
	{ "sigpending", (__NR_SYSCALL_BASE + 73) },
	{ "sigprocmask", (__NR_SYSCALL_BASE + 126) },
	{ "sigreturn", (__NR_SYSCALL_BASE + 119) },
	{ "sigsuspend", (__NR_SYSCALL_BASE + 72) },
	{ "socket", (__NR_SYSCALL_BASE + 281) },
	{ "socketcall", (__NR_SYSCALL_BASE + 102) },
	{ "socketpair", (__NR_SYSCALL_BASE + 288) },
	{ "splice", (__NR_SYSCALL_BASE + 340) },
	{ "ssetmask", __PNR_ssetmask },
	{ "stat", (__NR_SYSCALL_BASE + 106) },
	{ "stat64", (__NR_SYSCALL_BASE + 195) },
	{ "statfs", (__NR_SYSCALL_BASE + 99) },
	{ "statfs64", (__NR_SYSCALL_BASE + 266) },
	{ "stime", (__NR_SYSCALL_BASE + 25) },
	{ "stty", __PNR_stty },
	{ "swapoff", (__NR_SYSCALL_BASE + 115) },
	{ "swapon", (__NR_SYSCALL_BASE + 87) },
	{ "symlink", (__NR_SYSCALL_BASE + 83) },
	{ "symlinkat", (__NR_SYSCALL_BASE + 331) },
	{ "sync", (__NR_SYSCALL_BASE + 36) },
	{ "sync_file_range", __PNR_sync_file_range },
	{ "sync_file_range2", (__NR_SYSCALL_BASE + 341) },
	{ "syncfs", (__NR_SYSCALL_BASE + 373) },
	{ "syscall", (__NR_SYSCALL_BASE + 113) },
	{ "sysfs", (__NR_SYSCALL_BASE + 135) },
	{ "sysinfo", (__NR_SYSCALL_BASE + 116) },
	{ "syslog", (__NR_SYSCALL_BASE + 103) },
	{ "sysmips", __PNR_sysmips },
	{ "tee", (__NR_SYSCALL_BASE + 342) },
	{ "tgkill", (__NR_SYSCALL_BASE + 268) },
	{ "time", (__NR_SYSCALL_BASE + 13) },
	{ "timer_create", (__NR_SYSCALL_BASE + 257) },
	{ "timer_delete", (__NR_SYSCALL_BASE + 261) },
	{ "timer_getoverrun", (__NR_SYSCALL_BASE + 260) },
	{ "timer_gettime", (__NR_SYSCALL_BASE + 259) },
	{ "timer_settime", (__NR_SYSCALL_BASE + 258) },
	{ "timerfd", __PNR_timerfd },
	{ "timerfd_create", (__NR_SYSCALL_BASE + 350) },
	{ "timerfd_gettime", (__NR_SYSCALL_BASE + 354) },
	{ "timerfd_settime", (__NR_SYSCALL_BASE + 353) },
	{ "times", (__NR_SYSCALL_BASE + 43) },
	{ "tkill", (__NR_SYSCALL_BASE + 238) },
	{ "truncate", (__NR_SYSCALL_BASE + 92) },
	{ "truncate64", (__NR_SYSCALL_BASE + 193) },
	{ "tuxcall", __PNR_tuxcall },
	{ "ugetrlimit", (__NR_SYSCALL_BASE + 191) },
	{ "ulimit", __PNR_ulimit },
	{ "umask", (__NR_SYSCALL_BASE + 60) },
	{ "umount", (__NR_SYSCALL_BASE + 22) },
	{ "umount2", (__NR_SYSCALL_BASE + 52) },
	{ "uname", (__NR_SYSCALL_BASE + 122) },
	{ "unlink", (__NR_SYSCALL_BASE + 10) },
	{ "unlinkat", (__NR_SYSCALL_BASE + 328) },
	{ "unshare", (__NR_SYSCALL_BASE + 337) },
	{ "uselib", (__NR_SYSCALL_BASE + 86) },
	{ "ustat", (__NR_SYSCALL_BASE + 62) },
	{ "utime", (__NR_SYSCALL_BASE + 30) },
	{ "utimensat", (__NR_SYSCALL_BASE + 348) },
	{ "utimes", (__NR_SYSCALL_BASE + 269) },
	{ "vfork", (__NR_SYSCALL_BASE + 190) },
	{ "vhangup", (__NR_SYSCALL_BASE + 111) },
	{ "vm86", __PNR_vm86 },
	{ "vm86old", __PNR_vm86old },
	{ "vmsplice", (__NR_SYSCALL_BASE + 343) },
	{ "vserver", (__NR_SYSCALL_BASE + 313) },
	{ "wait4", (__NR_SYSCALL_BASE + 114) },
	{ "waitid", (__NR_SYSCALL_BASE + 280) },
	{ "waitpid", __PNR_waitpid },
	{ "write", (__NR_SYSCALL_BASE +  4) },
	{ "writev", (__NR_SYSCALL_BASE + 146) },
	{ NULL, __NR_SCMP_ERROR },
};

/**
 * Resolve a syscall name to a number
 * @param name the syscall name
 *
 * Resolve the given syscall name to the syscall number using the syscall table.
 * Returns the syscall number on success, including negative pseudo syscall
 * numbers; returns __NR_SCMP_ERROR on failure.
 *
 */
int arm_syscall_resolve_name(const char *name)
{
	unsigned int iter;
	const struct arch_syscall_def *table = arm_syscall_table;

	/* XXX - plenty of room for future improvement here */
	for (iter = 0; table[iter].name != NULL; iter++) {
		if (strcmp(name, table[iter].name) == 0)
			return table[iter].num;
	}

	return __NR_SCMP_ERROR;
}

/**
 * Resolve a syscall number to a name
 * @param num the syscall number
 *
 * Resolve the given syscall number to the syscall name using the syscall table.
 * Returns a pointer to the syscall name string on success, including pseudo
 * syscall names; returns NULL on failure.
 *
 */
const char *arm_syscall_resolve_num(int num)
{
	unsigned int iter;
	const struct arch_syscall_def *table = arm_syscall_table;

	/* XXX - plenty of room for future improvement here */
	for (iter = 0; table[iter].num != __NR_SCMP_ERROR; iter++) {
		if (num == table[iter].num)
			return table[iter].name;
	}

	return NULL;
}

/**
 * Iterate through the syscall table and return the syscall name
 * @param spot the offset into the syscall table
 *
 * Return the syscall name at position @spot or NULL on failure.  This function
 * should only ever be used internally by libseccomp.
 *
 */
const char *arm_syscall_iterate_name(unsigned int spot)
{
	/* XXX - no safety checks here */
	return arm_syscall_table[spot].name;
}
