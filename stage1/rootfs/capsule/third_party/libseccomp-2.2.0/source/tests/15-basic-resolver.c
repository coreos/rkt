/**
 * Seccomp Library test program
 *
 * Copyright (c) 2012 Red Hat <pmoore@redhat.com>
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

#include <errno.h>
#include <string.h>

#include <seccomp.h>

int main(int argc, char *argv[])
{
	char *name;

	if (seccomp_syscall_resolve_name("open") != __NR_open)
		return 1;
	if (seccomp_syscall_resolve_name("socket") != __NR_socket)
		return 1;
	if (seccomp_syscall_resolve_name("INVALID") != __NR_SCMP_ERROR)
		return 1;

	if (seccomp_syscall_resolve_name_arch(SCMP_ARCH_NATIVE,
					      "open") != __NR_open)
		return 1;
	if (seccomp_syscall_resolve_name_arch(SCMP_ARCH_NATIVE,
					      "socket") != __NR_socket)
		return 1;
	if (seccomp_syscall_resolve_name_arch(SCMP_ARCH_NATIVE,
					      "INVALID") != __NR_SCMP_ERROR)
		return 1;

	name = seccomp_syscall_resolve_num_arch(SCMP_ARCH_NATIVE, __NR_open);
	if (name == NULL || strcmp(name, "open") != 0)
		return 1;
	name = seccomp_syscall_resolve_num_arch(SCMP_ARCH_NATIVE, __NR_socket);
	if (name == NULL || strcmp(name, "socket") != 0)
		return 1;
	name = seccomp_syscall_resolve_num_arch(SCMP_ARCH_NATIVE,
						__NR_SCMP_ERROR);
	if (name != NULL)
		return 1;

	return 0;
}
